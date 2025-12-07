package main

import (
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

	// Create wrapper script that starts Nomad
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
	fmt.Println("  nomad node status     # List Nomad nodes")
	fmt.Println("  nomad service list    # List registered services")
	fmt.Println("\nNomad UI:  http://127.0.0.1:4646")
	fmt.Println("Transport encryption provided by Tailscale WireGuard")

	return nil
}
