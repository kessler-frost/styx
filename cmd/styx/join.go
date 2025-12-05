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

	// Verify server is reachable
	fmt.Printf("Checking server at %s...\n", serverIP)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:4646/v1/agent/health", serverIP))
	if err != nil {
		return fmt.Errorf("cannot reach server at %s:4646: %w", serverIP, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server at %s is not healthy (status %d)", serverIP, resp.StatusCode)
	}
	fmt.Println("Server is reachable and healthy")

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
	fmt.Printf("Writing config to: %s\n", configPath)
	if err := config.WriteConfig(configPath, configContent); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
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
		Program:    nomadPath,
		Args:       []string{"agent", "-config=" + configDir},
		LogPath:    filepath.Join(logDir, "nomad.log"),
		ErrLogPath: filepath.Join(logDir, "nomad-error.log"),
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
	if err := waitForNomadHealth(30 * time.Second); err != nil {
		return fmt.Errorf("nomad failed to start: %w\nCheck logs at %s", err, filepath.Join(logDir, "nomad.log"))
	}

	// Wait a bit for client to register with server
	fmt.Println("Waiting for client to register with server...")
	time.Sleep(5 * time.Second)

	fmt.Println("\nSuccessfully joined the cluster!")
	fmt.Printf("Server: %s\n", serverIP)
	fmt.Println("\nCheck status with:")
	fmt.Println("  nomad node status     # List all nodes")

	return nil
}
