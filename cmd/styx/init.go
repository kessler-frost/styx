package main

import (
	"bytes"
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

	// Create directories
	dirs := []string{
		dataDir,
		configDir,
		logDir,
		pluginDir,
		consulDataDir,
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

	// Generate config
	var configContent string
	if serverMode {
		fmt.Println("Generating server configuration...")
		cfg := config.ServerConfig{
			DataDir:         dataDir,
			AdvertiseIP:     ip,
			BootstrapExpect: 1,
			PluginDir:       pluginDir,
		}
		configContent, err = config.GenerateServerConfig(cfg)
	} else {
		fmt.Println("Generating standalone client configuration...")
		cfg := config.ClientConfig{
			DataDir:     dataDir,
			AdvertiseIP: ip,
			Servers:     []string{ip}, // Point to self for standalone
			PluginDir:   pluginDir,
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
		}
		consulConfigContent, err = config.GenerateConsulServerConfig(consulCfg)
	} else {
		fmt.Println("Generating Consul client configuration...")
		consulCfg := config.ConsulClientConfig{
			DataDir:     consulDataDir,
			AdvertiseIP: ip,
			Servers:     []string{ip}, // Point to self for standalone
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

	// Create wrapper script that starts both Consul and Nomad
	wrapperPath := filepath.Join(configDir, "styx-agent.sh")
	consulConfigFile := filepath.Join(configDir, "consul.hcl")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
# Styx agent wrapper - starts Consul and Nomad together
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
	fmt.Println("\nConsul UI available at: http://127.0.0.1:8500")

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
