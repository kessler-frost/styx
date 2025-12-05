package main

import (
	"bytes"
	cryptotls "crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kessler-frost/styx/internal/config"
	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/kessler-frost/styx/internal/network"
	"github.com/kessler-frost/styx/internal/tls"
	"github.com/kessler-frost/styx/internal/vault"
	"github.com/spf13/cobra"
)

var (
	serverMode bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Styx cluster",
	Long: `Initialize a new Styx cluster node.

Use --server to initialize as a server node that can coordinate the cluster.
Without --server, initializes as a standalone client (use 'styx join' instead
to join an existing cluster).`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&serverMode, "server", false, "Initialize as server node")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if already running and healthy
	if launchd.IsLoaded("com.styx.nomad") {
		client := &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &cryptotls.Config{InsecureSkipVerify: true},
			},
		}
		resp, err := client.Get("https://127.0.0.1:4646/v1/agent/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("Styx is already running and healthy")
			fmt.Println("Use 'styx status' to check cluster status")
			fmt.Println("Use 'styx stop' first if you want to reinitialize")
			return nil
		}
	}

	// Check for nomad binary
	nomadPath, err := exec.LookPath("nomad")
	if err != nil {
		return fmt.Errorf("nomad not found in PATH. Please install nomad first: brew install nomad")
	}
	fmt.Printf("Found nomad at: %s\n", nomadPath)

	// Check for consul binary
	consulPath, err := exec.LookPath("consul")
	if err != nil {
		return fmt.Errorf("consul not found in PATH. Please install consul first: brew install consul")
	}
	fmt.Printf("Found consul at: %s\n", consulPath)

	// Check for container CLI
	containerPath, err := exec.LookPath("container")
	if err != nil {
		return fmt.Errorf("container CLI not found. Please ensure macOS 26+ with Apple Containers is installed")
	}
	fmt.Printf("Found container CLI at: %s\n", containerPath)

	// Detect local IP
	ip, err := network.GetPreferredIP()
	if err != nil {
		return fmt.Errorf("failed to detect local IP: %w", err)
	}
	fmt.Printf("Detected local IP: %s\n", ip)

	// Check Tailscale status for Phase 4 networking
	tailscale := network.GetTailscaleInfo()
	if tailscale.Running {
		fmt.Printf("Tailscale connected: %s (%s)\n", tailscale.DNSName, tailscale.IP)
		fmt.Println("  Services will be reachable via Tailscale from other nodes")
	} else {
		fmt.Println("Tailscale not connected (cross-node networking will be limited)")
		fmt.Println("  Install Tailscale: https://tailscale.com/download")
	}

	// Create directories
	dirs := []string{
		dataDir,
		configDir,
		logDir,
		pluginDir,
		consulDataDir,
		certsDir,
		secretsDir,
		vaultDataDir,
	}

	for _, dir := range dirs {
		fmt.Printf("Creating directory: %s\n", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy plugin to plugin directory
	pluginSrc := filepath.Join(filepath.Dir(os.Args[0]), "..", "plugins", "apple-container")
	// Also check common build locations
	if _, err := os.Stat(pluginSrc); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		pluginSrc = filepath.Join(cwd, "plugins", "apple-container")
	}

	pluginDst := filepath.Join(pluginDir, "apple-container")
	if _, err := os.Stat(pluginSrc); err == nil {
		fmt.Printf("Copying plugin from %s to %s\n", pluginSrc, pluginDst)
		if err := copyFile(pluginSrc, pluginDst); err != nil {
			return fmt.Errorf("failed to copy plugin: %w", err)
		}
		if err := os.Chmod(pluginDst, 0755); err != nil {
			return fmt.Errorf("failed to set plugin permissions: %w", err)
		}
	} else {
		fmt.Printf("Warning: plugin not found at %s, assuming already installed\n", pluginSrc)
	}

	// Generate TLS certificates
	fmt.Println("Setting up TLS certificates...")
	datacenter := "dc1"
	region := "global"

	// Generate CAs and certs (for server mode, or check for existing)
	var consulCertPaths *tls.CertPaths
	var nomadCertPaths *tls.NomadCertPaths
	var gossipKey string

	if serverMode {
		// Server: generate Consul CA, server cert, and gossip key
		fmt.Println("Generating Consul Certificate Authority...")
		if err := tls.GenerateCA(certsDir); err != nil {
			return fmt.Errorf("failed to generate Consul CA: %w", err)
		}

		fmt.Println("Generating Consul server certificates...")
		consulCertPaths, err = tls.GenerateServerCert(certsDir, datacenter)
		if err != nil {
			return fmt.Errorf("failed to generate Consul server cert: %w", err)
		}

		fmt.Println("Generating gossip encryption key...")
		gossipKey, err = tls.GenerateGossipKey()
		if err != nil {
			return fmt.Errorf("failed to generate gossip key: %w", err)
		}
		if err := tls.SaveGossipKey(secretsDir, gossipKey); err != nil {
			return fmt.Errorf("failed to save gossip key: %w", err)
		}

		// Generate Nomad CA and server cert (separate from Consul)
		fmt.Println("Generating Nomad Certificate Authority...")
		if err := tls.GenerateNomadCA(certsDir); err != nil {
			return fmt.Errorf("failed to generate Nomad CA: %w", err)
		}

		fmt.Println("Generating Nomad server certificates...")
		nomadCertPaths, err = tls.GenerateNomadServerCert(certsDir, region)
		if err != nil {
			return fmt.Errorf("failed to generate Nomad server cert: %w", err)
		}
	} else {
		// Standalone client: generate client certs (reuse CAs if exist)
		consulCertPaths, err = tls.GetExistingCerts(certsDir, datacenter, false)
		if err != nil {
			// No existing certs, generate new ones for standalone mode
			fmt.Println("Generating Consul Certificate Authority...")
			if err := tls.GenerateCA(certsDir); err != nil {
				return fmt.Errorf("failed to generate Consul CA: %w", err)
			}

			fmt.Println("Generating Consul client certificates...")
			consulCertPaths, err = tls.GenerateClientCert(certsDir, datacenter)
			if err != nil {
				return fmt.Errorf("failed to generate Consul client cert: %w", err)
			}

			fmt.Println("Generating gossip encryption key...")
			gossipKey, err = tls.GenerateGossipKey()
			if err != nil {
				return fmt.Errorf("failed to generate gossip key: %w", err)
			}
			if err := tls.SaveGossipKey(secretsDir, gossipKey); err != nil {
				return fmt.Errorf("failed to save gossip key: %w", err)
			}
		} else {
			gossipKey, err = tls.LoadGossipKey(secretsDir)
			if err != nil {
				return fmt.Errorf("failed to load gossip key: %w", err)
			}
		}

		// Generate Nomad client certs
		nomadCertPaths, err = tls.GetExistingNomadCerts(certsDir, region, false)
		if err != nil {
			fmt.Println("Generating Nomad Certificate Authority...")
			if err := tls.GenerateNomadCA(certsDir); err != nil {
				return fmt.Errorf("failed to generate Nomad CA: %w", err)
			}

			fmt.Println("Generating Nomad client certificates...")
			nomadCertPaths, err = tls.GenerateNomadClientCert(certsDir, region)
			if err != nil {
				return fmt.Errorf("failed to generate Nomad client cert: %w", err)
			}
		}
	}
	fmt.Printf("TLS certificates ready in: %s\n", certsDir)

	// Generate config
	var configContent string
	if serverMode {
		fmt.Println("Generating server configuration...")
		cfg := config.ServerConfig{
			DataDir:         dataDir,
			AdvertiseIP:     ip,
			BootstrapExpect: 1,
			PluginDir:       pluginDir,
			CAFile:          nomadCertPaths.CAFile,
			CertFile:        nomadCertPaths.CertFile,
			KeyFile:         nomadCertPaths.KeyFile,
		}
		configContent, err = config.GenerateServerConfig(cfg)
	} else {
		fmt.Println("Generating standalone client configuration...")
		cfg := config.ClientConfig{
			DataDir:     dataDir,
			AdvertiseIP: ip,
			Servers:     []string{ip}, // Point to self for standalone
			PluginDir:   pluginDir,
			CAFile:      nomadCertPaths.CAFile,
			CertFile:    nomadCertPaths.CertFile,
			KeyFile:     nomadCertPaths.KeyFile,
		}
		configContent, err = config.GenerateClientConfig(cfg)
	}
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	configPath := filepath.Join(configDir, "nomad.hcl")
	fmt.Printf("Writing Nomad config to: %s\n", configPath)
	if err := config.WriteConfig(configPath, configContent); err != nil {
		return fmt.Errorf("failed to write nomad config: %w", err)
	}

	// Generate Consul config
	var consulConfigContent string
	if serverMode {
		fmt.Println("Generating Consul server configuration...")
		consulCfg := config.ConsulServerConfig{
			DataDir:         consulDataDir,
			AdvertiseIP:     ip,
			BootstrapExpect: 1,
			CAFile:          consulCertPaths.CAFile,
			CertFile:        consulCertPaths.CertFile,
			KeyFile:         consulCertPaths.KeyFile,
			GossipKey:       gossipKey,
		}
		consulConfigContent, err = config.GenerateConsulServerConfig(consulCfg)
	} else {
		fmt.Println("Generating Consul client configuration...")
		consulCfg := config.ConsulClientConfig{
			DataDir:     consulDataDir,
			AdvertiseIP: ip,
			Servers:     []string{ip}, // Point to self for standalone
			CAFile:      consulCertPaths.CAFile,
			CertFile:    consulCertPaths.CertFile,
			KeyFile:     consulCertPaths.KeyFile,
			GossipKey:   gossipKey,
		}
		consulConfigContent, err = config.GenerateConsulClientConfig(consulCfg)
	}
	if err != nil {
		return fmt.Errorf("failed to generate consul config: %w", err)
	}

	consulConfigPath := filepath.Join(configDir, "consul.hcl")
	fmt.Printf("Writing Consul config to: %s\n", consulConfigPath)
	if err := config.WriteConfig(consulConfigPath, consulConfigContent); err != nil {
		return fmt.Errorf("failed to write consul config: %w", err)
	}

	// Generate Vault config (server mode only)
	var vaultPath string
	var vaultConfigPath string
	if serverMode {
		vaultPath, err = exec.LookPath("vault")
		if err != nil {
			return fmt.Errorf("vault not found in PATH. Please install vault first: brew install vault")
		}
		fmt.Printf("Found vault at: %s\n", vaultPath)

		fmt.Println("Generating Vault configuration...")
		vaultCfg := config.VaultConfig{
			AdvertiseIP: ip,
		}
		vaultConfigContent, err := config.GenerateVaultConfig(vaultCfg)
		if err != nil {
			return fmt.Errorf("failed to generate vault config: %w", err)
		}

		vaultConfigPath = filepath.Join(configDir, "vault.hcl")
		fmt.Printf("Writing Vault config to: %s\n", vaultConfigPath)
		if err := config.WriteConfig(vaultConfigPath, vaultConfigContent); err != nil {
			return fmt.Errorf("failed to write vault config: %w", err)
		}
	}

	// Create wrapper script that starts Consul, Vault (if server), and Nomad
	wrapperPath := filepath.Join(configDir, "styx-agent.sh")
	consulConfigFile := filepath.Join(configDir, "consul.hcl")

	var wrapperContent string
	if serverMode {
		// Server mode: includes Vault
		wrapperContent = fmt.Sprintf(`#!/bin/bash
# Styx agent wrapper - starts Consul, Vault, and Nomad
set -e

cleanup() {
    echo "Stopping services..."
    kill $NOMAD_PID 2>/dev/null || true
    kill $VAULT_PID 2>/dev/null || true
    kill $CONSUL_PID 2>/dev/null || true
    exit 0
}

trap cleanup SIGTERM SIGINT

# Start Consul
"%s" agent -config-file="%s" &
CONSUL_PID=$!

# Wait for Consul to be healthy
echo "Waiting for Consul..."
for i in {1..30}; do
    if curl -s http://127.0.0.1:8500/v1/status/leader | grep -q .; then
        echo "Consul is ready"
        break
    fi
    sleep 1
done

# Start Vault
"%s" server -config="%s" &
VAULT_PID=$!

# Wait for Vault to be ready
echo "Waiting for Vault..."
for i in {1..30}; do
    if curl -s http://127.0.0.1:8200/v1/sys/health 2>/dev/null | grep -q .; then
        echo "Vault is ready"
        break
    fi
    sleep 1
done

# Start Nomad (uses workload identity for Vault, no token needed)
"%s" agent -config="%s/nomad.hcl" &
NOMAD_PID=$!

# Wait for either to exit
wait
`, consulPath, consulConfigFile, vaultPath, vaultConfigPath, nomadPath, configDir)
	} else {
		// Client mode: no Vault
		wrapperContent = fmt.Sprintf(`#!/bin/bash
# Styx agent wrapper - starts Consul and Nomad
set -e

cleanup() {
    echo "Stopping services..."
    kill $NOMAD_PID 2>/dev/null || true
    kill $CONSUL_PID 2>/dev/null || true
    exit 0
}

trap cleanup SIGTERM SIGINT

# Start Consul
"%s" agent -config-file="%s" &
CONSUL_PID=$!

# Wait for Consul to be healthy
echo "Waiting for Consul..."
for i in {1..30}; do
    if curl -s http://127.0.0.1:8500/v1/status/leader | grep -q .; then
        echo "Consul is ready"
        break
    fi
    sleep 1
done

# Start Nomad
"%s" agent -config="%s/nomad.hcl" &
NOMAD_PID=$!

# Wait for either to exit
wait
`, consulPath, consulConfigFile, nomadPath, configDir)
	}

	fmt.Printf("Writing wrapper script to: %s\n", wrapperPath)
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	// Setup DNS resolver for .consul domain (requires sudo)
	fmt.Println("Setting up DNS resolver for .consul domain...")
	if err := setupConsulDNS(); err != nil {
		fmt.Printf("Warning: failed to setup DNS resolver: %v\n", err)
		fmt.Println("You can manually create /etc/resolver/consul with:")
		fmt.Println("  nameserver 127.0.0.1")
		fmt.Println("  port 8600")
	}

	// Generate and write launchd plist (user agent)
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.styx.nomad.plist")
	fmt.Printf("Creating launchd plist at: %s\n", plistPath)

	// Ensure LaunchAgents directory exists
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistCfg := launchd.PlistConfig{
		Label:      "com.styx.nomad",
		Program:    "/bin/bash",
		Args:       []string{wrapperPath},
		LogPath:    filepath.Join(logDir, "styx.log"),
		ErrLogPath: filepath.Join(logDir, "styx-error.log"),
		WorkingDir: configDir,
	}
	if err := launchd.WritePlist(plistPath, plistCfg); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Unload if already loaded (ignore errors)
	if launchd.IsLoaded("com.styx.nomad") {
		fmt.Println("Unloading existing service...")
		_ = launchd.Unload(plistPath)
		time.Sleep(2 * time.Second)
	}

	// Load the service
	fmt.Println("Loading launchd service...")
	if err := launchd.Load(plistPath); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	// Wait for Consul to become healthy
	fmt.Println("Waiting for Consul to become healthy...")
	if err := waitForConsulHealth(30 * time.Second); err != nil {
		return fmt.Errorf("consul failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

	// Initialize and unseal Vault (server mode only)
	if serverMode {
		fmt.Println("Waiting for Vault to become ready...")
		if err := waitForVaultHealth(30 * time.Second); err != nil {
			return fmt.Errorf("vault failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
		}

		// Check if Vault needs initialization
		initialized, err := vault.IsInitialized()
		if err != nil {
			return fmt.Errorf("failed to check vault status: %w", err)
		}

		if !initialized {
			fmt.Println("Initializing Vault...")
			_, err = vault.Initialize(secretsDir)
			if err != nil {
				return fmt.Errorf("failed to initialize vault: %w", err)
			}
		}

		// Unseal if sealed
		sealed, _ := vault.IsSealed()
		if sealed {
			fmt.Println("Unsealing Vault...")
			if err := vault.Unseal(secretsDir); err != nil {
				return fmt.Errorf("failed to unseal vault: %w", err)
			}
		}

		// Setup Nomad integration with workload identities
		fmt.Println("Setting up Vault-Nomad integration...")
		nomadCAPath := filepath.Join(certsDir, "nomad-agent-ca.pem")
		if err := vault.SetupNomadIntegration(secretsDir, nomadCAPath); err != nil {
			fmt.Printf("Warning: failed to setup Vault-Nomad integration: %v\n", err)
			fmt.Println("You can set this up later with 'vault policy write' and 'vault token create'")
		}
	}

	// Wait for Nomad to become healthy
	// Nomad can take 40+ seconds to start due to plugin loading and client registration
	fmt.Println("Waiting for Nomad to become healthy...")
	if err := waitForNomadHealth(60 * time.Second); err != nil {
		return fmt.Errorf("nomad failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

	fmt.Println("\nStyx initialized successfully!")
	if serverMode {
		fmt.Println("\nServer is running. To add client nodes, run on other Macs:")
		fmt.Printf("  styx join %s\n", ip)
	}
	fmt.Println("\nCheck status with:")
	fmt.Println("  styx status           # Show Styx status")
	fmt.Println("  consul members        # List Consul members")
	fmt.Println("  nomad node status     # List Nomad nodes")
	if serverMode {
		fmt.Println("  vault status          # Show Vault status")
	}
	fmt.Println("\nConsul UI: http://127.0.0.1:8500 (HTTPS: https://127.0.0.1:8501)")
	if serverMode {
		fmt.Println("Vault UI:  http://127.0.0.1:8200/ui")
	}
	fmt.Println("TLS enabled - APIs secured with mTLS")

	return nil
}

func waitForNomadHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	// Use HTTPS since we have TLS enabled on Nomad HTTP API
	// Skip certificate verification for localhost health checks
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &cryptotls.Config{InsecureSkipVerify: true},
		},
	}

	for time.Now().Before(deadline) {
		resp, err := client.Get("https://127.0.0.1:4646/v1/agent/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
	fmt.Println()
	return fmt.Errorf("timeout waiting for nomad health")
}

func waitForConsulHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:8500/v1/status/leader")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
	fmt.Println()
	return fmt.Errorf("timeout waiting for consul health")
}

func waitForVaultHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:8200/v1/sys/health")
		if err == nil {
			resp.Body.Close()
			// Vault returns 200 (initialized, unsealed), 429 (unsealed, standby),
			// 472 (DR secondary), 473 (performance standby), 501 (not initialized), or 503 (sealed)
			// Any of these means Vault is responding
			if resp.StatusCode == 200 || resp.StatusCode == 429 || resp.StatusCode == 501 || resp.StatusCode == 503 {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
	fmt.Println()
	return fmt.Errorf("timeout waiting for vault health")
}

func setupConsulDNS() error {
	// Create /etc/resolver directory if it doesn't exist
	// Then create /etc/resolver/consul with DNS config
	resolverContent := "nameserver 127.0.0.1\nport 8600\n"

	// Use sudo to write to /etc/resolver/consul
	cmd := exec.Command("sudo", "mkdir", "-p", "/etc/resolver")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create /etc/resolver: %w", err)
	}

	cmd = exec.Command("sudo", "tee", "/etc/resolver/consul")
	cmd.Stdin = bytes.NewBufferString(resolverContent)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write /etc/resolver/consul: %w", err)
	}

	fmt.Println("DNS resolver configured for .consul domain")
	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
