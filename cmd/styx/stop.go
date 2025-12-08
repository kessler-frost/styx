package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/kessler-frost/styx/internal/services"
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
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.styx.nomad.plist")
	label := "com.styx.nomad"

	if !launchd.IsLoaded(label) {
		fmt.Println("Styx service is not running")
		return nil
	}

	fmt.Println("Stopping Styx service...")

	// Stop all Nomad jobs first so containers are properly cleaned up
	stopAllJobs()

	// Stop the service
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

// stopAllJobs stops all running Nomad jobs
func stopAllJobs() {
	client := services.DefaultClient()

	jobs, err := client.ListJobs()
	if err != nil {
		fmt.Printf("Warning: failed to list jobs: %v\n", err)
		return
	}

	if len(jobs) == 0 {
		return
	}

	fmt.Printf("Stopping %d job(s)...\n", len(jobs))

	for _, job := range jobs {
		if job.Status == "dead" {
			continue
		}
		fmt.Printf("  Stopping job: %s\n", job.ID)
		if err := client.StopJob(job.ID); err != nil {
			fmt.Printf("  Warning: failed to stop job %s: %v\n", job.ID, err)
		}
	}

	// Wait for jobs to stop and containers to be cleaned up
	time.Sleep(3 * time.Second)
}
