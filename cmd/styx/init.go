package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kessler-frost/styx/internal/config"
	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/kessler-frost/styx/internal/network"
	"github.com/kessler-frost/styx/internal/services"
	"github.com/kessler-frost/styx/internal/tailserve"
	"github.com/kessler-frost/styx/internal/vault"
	"github.com/spf13/cobra"
)

var (
	serveMode bool
	joinIP    string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Start or join a Styx cluster",
	Long: `Initialize Styx by starting a server or joining an existing cluster.

By default, init auto-discovers servers on your Tailscale network:
  - If no servers found, prompts to start one
  - If one server found, auto-joins it
  - If multiple servers found, prompts for selection

Flags:
  --serve       Force server mode (starts Nomad + Vault + platform services)
  --join <ip>   Join a specific server by IP address`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&serveMode, "serve", false, "Force server mode")
	initCmd.Flags().StringVar(&joinIP, "join", "", "Join a specific server by IP")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if already running
	if launchd.IsLoaded("com.styx.nomad") {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:4646/v1/agent/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("Styx is already running and healthy")
			fmt.Println("Use 'styx status' to check cluster status")
			fmt.Println("Use 'styx stop' first if you want to reinitialize")
			return nil
		}
	}

	// Determine mode
	if serveMode {
		return runServer()
	}

	if joinIP != "" {
		return runClient(joinIP)
	}

	// Auto-discover mode
	return runAutoDiscover()
}

// runAutoDiscover probes Tailscale peers for Nomad servers
func runAutoDiscover() error {
	// Check Tailscale status
	tsInfo := network.GetTailscaleInfo()
	if !tsInfo.Running {
		fmt.Println("Tailscale is not running.")
		fmt.Println()
		fmt.Println("To auto-discover servers, install and connect Tailscale:")
		fmt.Println("  https://tailscale.com/download")
		fmt.Println()
		fmt.Println("Or use manual commands:")
		fmt.Println("  styx init --serve       Start a server on this machine")
		fmt.Println("  styx init --join <ip>   Join an existing server")
		return nil
	}

	fmt.Printf("Tailscale connected: %s (%s)\n", tsInfo.DNSName, tsInfo.IP)
	fmt.Println("Discovering Nomad servers on Tailscale network...")

	servers := network.DiscoverNomadServers(3 * time.Second)

	// No servers found - prompt to start one
	if len(servers) == 0 {
		fmt.Println()
		fmt.Println("No Nomad servers found on your Tailscale network.")
		fmt.Println()

		if promptYesNo("Start a server on this machine?") {
			return runServer()
		}

		fmt.Println()
		fmt.Println("Run 'styx init --serve' to start a server on this machine.")
		return nil
	}

	// Single server found - auto-join
	if len(servers) == 1 {
		server := servers[0]
		fmt.Printf("\nFound server: %s (%s)\n", server.Hostname, server.IP)
		fmt.Println("Joining cluster...")
		fmt.Println()
		return runClient(server.IP)
	}

	// Multiple servers found - prompt for selection
	fmt.Printf("\nFound %d Nomad servers:\n", len(servers))
	fmt.Println()
	for i, s := range servers {
		fmt.Printf("  [%d] %s (%s)\n", i+1, s.Hostname, s.IP)
	}
	fmt.Println()

	selected := promptServerSelection(len(servers))
	if selected < 0 {
		return nil
	}

	server := servers[selected]
	fmt.Println("Joining cluster...")
	fmt.Println()
	return runClient(server.IP)
}

// runServer starts Styx in server mode (Nomad + Vault + platform services)
func runServer() error {
	// Check for nomad binary
	nomadPath, err := exec.LookPath("nomad")
	if err != nil {
		return fmt.Errorf("nomad not found in PATH. Please install nomad first: brew install nomad")
	}
	fmt.Printf("Found nomad at: %s\n", nomadPath)

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

	// Check Tailscale status for networking
	tailscale := network.GetTailscaleInfo()
	if tailscale.Running {
		fmt.Printf("Tailscale connected: %s (%s)\n", tailscale.DNSName, tailscale.IP)
		fmt.Println("  Services will be reachable via Tailscale from other nodes")
		fmt.Println("  Transport encryption provided by Tailscale WireGuard")
	} else {
		fmt.Println("Tailscale not connected (cross-node networking will be limited)")
		fmt.Println("  Install Tailscale: https://tailscale.com/download")
	}

	// Create container network for service-to-service communication
	fmt.Println("Creating container network...")
	if err := network.EnsureStyxNetwork(); err != nil {
		return fmt.Errorf("failed to create container network: %w", err)
	}
	fmt.Printf("Container network ready: %s (%s)\n", network.StyxNetworkName, network.StyxNetworkSubnet)

	// Create directories
	dirs := []string{
		dataDir,
		configDir,
		logDir,
		pluginDir,
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

	// Generate Nomad server config
	fmt.Println("Generating server configuration...")
	cfg := config.ServerConfig{
		DataDir:         dataDir,
		AdvertiseIP:     ip,
		BootstrapExpect: 1,
		PluginDir:       pluginDir,
		CPUTotalCompute: config.GetCPUTotalCompute(),
	}
	configContent, err := config.GenerateServerConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	configPath := filepath.Join(configDir, "nomad.hcl")
	fmt.Printf("Writing Nomad config to: %s\n", configPath)
	if err := config.WriteConfig(configPath, configContent); err != nil {
		return fmt.Errorf("failed to write nomad config: %w", err)
	}

	// Generate Vault config
	vaultPath, err := exec.LookPath("vault")
	if err != nil {
		return fmt.Errorf("vault not found in PATH. Please install vault first: brew install vault")
	}
	fmt.Printf("Found vault at: %s\n", vaultPath)

	hostname, _ := os.Hostname()
	nodeID := hostname
	if nodeID == "" {
		nodeID = "node1"
	}

	fmt.Println("Generating Vault configuration (Raft storage)...")
	vaultCfg := config.VaultConfig{
		DataDir:     vaultDataDir,
		NodeID:      nodeID,
		AdvertiseIP: ip,
	}
	vaultConfigContent, err := config.GenerateVaultConfig(vaultCfg)
	if err != nil {
		return fmt.Errorf("failed to generate vault config: %w", err)
	}

	vaultConfigPath := filepath.Join(configDir, "vault.hcl")
	fmt.Printf("Writing Vault config to: %s\n", vaultConfigPath)
	if err := config.WriteConfig(vaultConfigPath, vaultConfigContent); err != nil {
		return fmt.Errorf("failed to write vault config: %w", err)
	}

	// Create wrapper script that starts Vault and Nomad
	wrapperPath := filepath.Join(configDir, "styx-agent.sh")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
# Styx agent wrapper - starts Vault and Nomad
set -e

cleanup() {
    echo "Stopping services..."
    kill $NOMAD_PID 2>/dev/null || true
    kill $VAULT_PID 2>/dev/null || true
    exit 0
}

trap cleanup SIGTERM SIGINT

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

# Start Nomad
"%s" agent -config="%s/nomad.hcl" &
NOMAD_PID=$!

# Wait for either to exit
wait
`, vaultPath, vaultConfigPath, nomadPath, configDir)

	fmt.Printf("Writing wrapper script to: %s\n", wrapperPath)
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	// Generate and write launchd plist (user agent)
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.styx.nomad.plist")
	fmt.Printf("Creating launchd plist at: %s\n", plistPath)

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

	// Unload if already loaded
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

	// Initialize and unseal Vault
	fmt.Println("Waiting for Vault to become ready...")
	if err := waitForVaultHealth(30 * time.Second); err != nil {
		return fmt.Errorf("vault failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

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

	sealed, _ := vault.IsSealed()
	if sealed {
		fmt.Println("Unsealing Vault...")
		if err := vault.Unseal(secretsDir); err != nil {
			return fmt.Errorf("failed to unseal vault: %w", err)
		}
	}

	fmt.Println("Waiting for Vault to become active...")
	if err := waitForVaultActive(60 * time.Second); err != nil {
		return fmt.Errorf("vault failed to become active: %w", err)
	}

	fmt.Println("Setting up Vault-Nomad integration...")
	if err := vault.SetupNomadIntegration(secretsDir); err != nil {
		fmt.Printf("Warning: failed to setup Vault-Nomad integration: %v\n", err)
		fmt.Println("You can set this up later with 'vault policy write' and 'vault token create'")
	}

	// Wait for Nomad to become healthy
	fmt.Println("Waiting for Nomad to become healthy...")
	if err := waitForNomadHealth(60 * time.Second); err != nil {
		return fmt.Errorf("nomad failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

	// Deploy platform services
	fmt.Println("\nDeploying platform services...")
	if err := services.DeployAll(); err != nil {
		return fmt.Errorf("failed to deploy platform services: %w", err)
	}

	// Enable Tailscale Serve for HTTPS ingress
	fmt.Println("\nEnabling Tailscale Serve for HTTPS ingress...")
	if err := tailserve.Enable(); err != nil {
		fmt.Printf("  Warning: failed to enable Tailscale Serve: %v\n", err)
		fmt.Println("  Traefik is still available at http://localhost:4200")
	}

	// Get Tailscale info for displaying ingress URL
	tsInfo := network.GetTailscaleInfo()

	fmt.Println("\nStyx server started!")
	fmt.Println("\nPlatform services:")
	fmt.Println("  NATS:      nats://localhost:4222")
	fmt.Println("  Dragonfly: redis://localhost:6379")
	if tsInfo.Running {
		fmt.Printf("  Traefik:   https://%s (ingress)\n", tsInfo.DNSName)
	} else {
		fmt.Println("  Traefik:   http://localhost:4200 (ingress)")
	}
	fmt.Println("             http://localhost:4201 (dashboard)")
	fmt.Println("\nTo add more nodes, run on other Macs:")
	fmt.Println("  styx init")
	fmt.Println("\nCheck status with:")
	fmt.Println("  styx status           # Show Styx status")
	fmt.Println("  styx services         # Show platform services")
	fmt.Println("  nomad node status     # List Nomad nodes")
	fmt.Println("\nVault UI:  http://127.0.0.1:8200/ui")
	fmt.Println("Nomad UI:  http://127.0.0.1:4646")

	return nil
}

// runClient joins an existing Styx cluster
func runClient(serverIP string) error {
	// Check for nomad binary
	nomadPath, err := exec.LookPath("nomad")
	if err != nil {
		return fmt.Errorf("nomad not found in PATH. Please install nomad first: brew install nomad")
	}
	fmt.Printf("Found nomad at: %s\n", nomadPath)

	// Check for container CLI
	containerPath, err := exec.LookPath("container")
	if err != nil {
		return fmt.Errorf("container CLI not found. Please ensure macOS 26+ with Apple Containers is installed")
	}
	fmt.Printf("Found container CLI at: %s\n", containerPath)

	// Verify Nomad server is reachable
	fmt.Printf("Checking Nomad server at %s...\n", serverIP)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:4646/v1/agent/health", serverIP))
	if err != nil {
		return fmt.Errorf("cannot reach Nomad server at %s:4646: %w", serverIP, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Nomad server at %s is not healthy (status %d)", serverIP, resp.StatusCode)
	}
	fmt.Println("Nomad server is reachable and healthy")

	// Detect local IP
	ip, err := network.GetPreferredIP()
	if err != nil {
		return fmt.Errorf("failed to detect local IP: %w", err)
	}
	fmt.Printf("Detected local IP: %s\n", ip)

	// Check Tailscale status
	tailscale := network.GetTailscaleInfo()
	if tailscale.Running {
		fmt.Printf("Tailscale connected: %s (%s)\n", tailscale.DNSName, tailscale.IP)
		fmt.Println("  Services will be reachable via Tailscale from other nodes")
	} else {
		fmt.Println("Tailscale not connected (cross-node networking will be limited)")
	}

	// Create container network for service-to-service communication
	fmt.Println("Creating container network...")
	if err := network.EnsureStyxNetwork(); err != nil {
		return fmt.Errorf("failed to create container network: %w", err)
	}
	fmt.Printf("Container network ready: %s (%s)\n", network.StyxNetworkName, network.StyxNetworkSubnet)

	// Create directories
	dirs := []string{
		dataDir,
		configDir,
		logDir,
		pluginDir,
	}

	for _, dir := range dirs {
		fmt.Printf("Creating directory: %s\n", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy plugin to plugin directory
	pluginSrc := filepath.Join(filepath.Dir(os.Args[0]), "..", "plugins", "apple-container")
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

	// Generate client config
	fmt.Println("Generating client configuration...")
	cfg := config.ClientConfig{
		DataDir:         dataDir,
		AdvertiseIP:     ip,
		Servers:         []string{serverIP},
		PluginDir:       pluginDir,
		CPUTotalCompute: config.GetCPUTotalCompute(),
	}
	configContent, err := config.GenerateClientConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	configPath := filepath.Join(configDir, "nomad.hcl")
	fmt.Printf("Writing Nomad config to: %s\n", configPath)
	if err := config.WriteConfig(configPath, configContent); err != nil {
		return fmt.Errorf("failed to write nomad config: %w", err)
	}

	// Create wrapper script
	wrapperPath := filepath.Join(configDir, "styx-agent.sh")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
# Styx agent wrapper - starts Nomad
set -e

cleanup() {
    echo "Stopping services..."
    kill $NOMAD_PID 2>/dev/null || true
    exit 0
}

trap cleanup SIGTERM SIGINT

# Start Nomad
"%s" agent -config="%s/nomad.hcl" &
NOMAD_PID=$!

# Wait for exit
wait
`, nomadPath, configDir)

	fmt.Printf("Writing wrapper script to: %s\n", wrapperPath)
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	// Generate and write launchd plist
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.styx.nomad.plist")
	fmt.Printf("Creating launchd plist at: %s\n", plistPath)

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

	// Unload if already loaded
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

	// Wait for Nomad to become healthy locally
	fmt.Println("Waiting for Nomad client to start...")
	if err := waitForNomadHealth(60 * time.Second); err != nil {
		return fmt.Errorf("nomad failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

	// Wait for client to register with server
	fmt.Println("Waiting for client to register with server...")
	time.Sleep(5 * time.Second)

	fmt.Println("\nSuccessfully joined the cluster!")
	fmt.Printf("Server: %s\n", serverIP)
	fmt.Println("\nCheck status with:")
	fmt.Println("  styx status           # Show Styx status")
	fmt.Println("  nomad node status     # List Nomad nodes")
	fmt.Println("\nNomad UI:  http://127.0.0.1:4646")

	return nil
}

func waitForNomadHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:4646/v1/agent/health")
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

func waitForVaultHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:8200/v1/sys/health")
		if err == nil {
			resp.Body.Close()
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

func waitForVaultActive(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:8200/v1/sys/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
	fmt.Println()
	return fmt.Errorf("timeout waiting for vault to become active")
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

func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [y/N]: ", question)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func promptServerSelection(count int) int {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Select a server [1-%d] or 'q' to quit: ", count)

	input, err := reader.ReadString('\n')
	if err != nil {
		return -1
	}

	input = strings.TrimSpace(input)

	if input == "q" || input == "Q" || input == "" {
		return -1
	}

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > count {
		fmt.Println("Invalid selection")
		return -1
	}

	return num - 1
}
