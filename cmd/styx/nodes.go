package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/spf13/cobra"
)

var nodesJSON bool

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List cluster nodes",
	Long:  `Display all Nomad client nodes in the cluster.`,
	RunE:  runNodes,
}

func init() {
	nodesCmd.Flags().BoolVar(&nodesJSON, "json", false, "Output in JSON format")
	rootCmd.AddCommand(nodesCmd)
}

func runNodes(cmd *cobra.Command, args []string) error {
	client := api.NewClient()
	nodes, err := client.GetNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	if nodes == nil {
		if nodesJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("Nomad is not running. Start Styx first with 'styx init'")
		return nil
	}

	if nodesJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(nodes)
	}

	if len(nodes) == 0 {
		fmt.Println("No nodes in cluster")
		return nil
	}

	fmt.Println("Nodes")
	fmt.Println("-----")
	fmt.Println()

	for _, node := range nodes {
		statusIcon := getNodeStatusIcon(node.Status, node.Drain)
		fmt.Printf("  %s %-20s %-15s %s\n", statusIcon, node.Name, node.Address, node.Datacenter)
	}

	return nil
}

func getNodeStatusIcon(status string, drain bool) string {
	if drain {
		return "[draining]"
	}
	switch status {
	case "ready":
		return "[ready]"
	case "down":
		return "[down]"
	default:
		return "[" + status + "]"
	}
}
