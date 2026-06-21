package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Stop all Nomad jobs first so containers are properly cleaned up.
	// Job-stop failures are surfaced but non-fatal: we still want to unload the
	// service so the node is fully stopped.
	if err := stopAllJobs(); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

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

// stopAllJobs stops all running Nomad jobs. It returns an aggregated error
// describing any jobs that failed to stop so callers can surface the failure
// instead of silently discarding it.
func stopAllJobs() error {
	client := services.DefaultClient()

	jobs, err := client.ListJobs()
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	if len(jobs) == 0 {
		return nil
	}

	fmt.Printf("Stopping %d job(s)...\n", len(jobs))

	var failed []string
	for _, job := range jobs {
		if job.Status == "dead" {
			continue
		}
		fmt.Printf("  Stopping job: %s\n", job.ID)
		if err := client.StopJob(job.ID); err != nil {
			fmt.Printf("  Warning: failed to stop job %s: %v\n", job.ID, err)
			failed = append(failed, job.ID)
		}
	}

	// Wait for jobs to stop and containers to be cleaned up
	time.Sleep(3 * time.Second)

	if len(failed) > 0 {
		return fmt.Errorf("failed to stop %d job(s): %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}
