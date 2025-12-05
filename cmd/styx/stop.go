package main

import (
	"fmt"
	"time"

	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Styx services",
	Long:  `Stop the Styx/Nomad service running on this node.`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	plistPath := "/Library/LaunchDaemons/com.styx.nomad.plist"
	label := "com.styx.nomad"

	if !launchd.IsLoaded(label) {
		fmt.Println("Styx service is not running")
		return nil
	}

	fmt.Println("Stopping Styx service...")

	// Stop the service first
	if err := launchd.Stop(label); err != nil {
		fmt.Printf("Warning: failed to stop service: %v\n", err)
	}

	// Wait for graceful shutdown
	time.Sleep(2 * time.Second)

	// Unload the service
	if err := launchd.Unload(plistPath); err != nil {
		return fmt.Errorf("failed to unload service: %w", err)
	}

	fmt.Println("Styx service stopped")
	return nil
}
