package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/spf13/cobra"
)

var jobsJSON bool

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "List Nomad jobs",
	Long:  `Display all running Nomad jobs and their allocations.`,
	RunE:  runJobs,
}

func init() {
	jobsCmd.Flags().BoolVar(&jobsJSON, "json", false, "Output in JSON format")
	rootCmd.AddCommand(jobsCmd)
}

func runJobs(cmd *cobra.Command, args []string) error {
	client := api.NewClient()
	jobs, err := client.GetJobs()
	if err != nil {
		return fmt.Errorf("failed to get jobs: %w", err)
	}

	if jobs == nil {
		if jobsJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("Nomad is not running. Start Styx first with 'styx init'")
		return nil
	}

	if jobsJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs running")
		return nil
	}

	fmt.Println("Jobs")
	fmt.Println("----")
	fmt.Println()

	for _, job := range jobs {
		statusIcon := getJobStatusIcon(job.Status)
		fmt.Printf("  %s %-20s %-10s %s\n", statusIcon, job.Name, job.Type, job.Status)

		// Show allocations
		for _, alloc := range job.Allocations {
			allocIcon := getAllocStatusIcon(alloc.ClientStatus)
			fmt.Printf("      %s %s on %s\n", allocIcon, alloc.ID[:8], alloc.NodeName)
		}
	}

	return nil
}

func getJobStatusIcon(status string) string {
	switch status {
	case "running":
		return "[running]"
	case "pending":
		return "[pending]"
	case "dead":
		return "[dead]"
	default:
		return "[" + status + "]"
	}
}

func getAllocStatusIcon(status string) string {
	switch status {
	case "running":
		return ">"
	case "pending":
		return "~"
	case "complete":
		return "+"
	case "failed":
		return "!"
	default:
		return "-"
	}
}
