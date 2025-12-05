package main

import (
	"bytes"
	"fmt"
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

var joinCmd = &cobra.Command{
	Use:   "join <server-ip>",
	Short: "Join an existing Styx cluster",
	Long: `Join an existing Styx cluster as a client node.

The server-ip argument should be the IP address of an existing Styx server node.
This node will register with the server and be available to run workloads.`,
	Args: cobra.ExactArgs(1),
	RunE: runJoin,
}

func init() {
	rootCmd.AddCommand(joinCmd)
}

func runJoin(cmd *cobra.Command, args []string) error {
	serverIP := args[0]

	// Check if already running and healthy
	if launchd.IsLoaded("com.styx.nomad") {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:4646/v1/agent/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("Styx is already running and healthy")
			fmt.Println("Use 'styx status' to check cluster status")
			fmt.Println("Use 'styx stop' first if you want to rejoin")
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

	// Verify Consul server is reachable
	fmt.Printf("Checking Consul server at %s...\n", serverIP)
	resp, err = client.Get(fmt.Sprintf("http://%s:8500/v1/status/leader", serverIP))
	if err != nil {
		return fmt.Errorf("cannot reach Consul server at %s:8500: %w", serverIP, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Consul server at %s is not healthy (status %d)", serverIP, resp.StatusCode)
	}
	fmt.Println("Consul server is reachable and healthy")

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
		DataDir:     dataDir,
		AdvertiseIP: ip,
		Servers:     []string{serverIP},
		PluginDir:   pluginDir,
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

	// Generate Consul client config
	fmt.Println("Generating Consul client configuration...")
	consulCfg := config.ConsulClientConfig{
		DataDir:     consulDataDir,
		AdvertiseIP: ip,
		Servers:     []string{serverIP},
	}
	consulConfigContent, err := config.GenerateConsulClientConfig(consulCfg)
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
	if err := setupConsulDNSJoin(); err != nil {
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

	// Wait for Consul to become healthy
	fmt.Println("Waiting for Consul to become healthy...")
	if err := waitForConsulHealthJoin(30 * time.Second); err != nil {
		return fmt.Errorf("consul failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

	// Wait for Nomad to become healthy locally
	fmt.Println("Waiting for Nomad client to start...")
	if err := waitForNomadHealth(60 * time.Second); err != nil {
		return fmt.Errorf("nomad failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "styx.log"))
	}

	// Wait a bit for client to register with server
	fmt.Println("Waiting for client to register with server...")
	time.Sleep(5 * time.Second)

	fmt.Println("\nSuccessfully joined the cluster!")
	fmt.Printf("Server: %s\n", serverIP)
	fmt.Println("\nCheck status with:")
	fmt.Println("  styx status           # Show Styx status")
	fmt.Println("  consul members        # List Consul members")
	fmt.Println("  nomad node status     # List Nomad nodes")
	fmt.Println("\nConsul UI available at: http://127.0.0.1:8500")

	return nil
}

func waitForConsulHealthJoin(timeout time.Duration) error {
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

func setupConsulDNSJoin() error {
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
