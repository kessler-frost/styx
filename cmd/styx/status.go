package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Styx cluster status",
	Long:  `Display the current status of the Styx/Nomad service and cluster connectivity.`,
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	client := api.NewClient()
	status := client.GetClusterStatus()

	if statusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	return printStatusHuman(status)
}

func printStatusHuman(status api.ClusterStatus) error {
	fmt.Println("Styx Status")
	fmt.Println("-----------")

	// Service status
	if status.Service == "stopped" {
		fmt.Println("Service:     stopped")
		fmt.Println()
		fmt.Println("Styx is not running.")
		fmt.Println("  To start a server:  styx init --serve")
		fmt.Println("  To join a cluster:  styx init --join <ip>")
		return nil
	}
	fmt.Println("Service:     running")

	// Vault status
	switch status.Vault.Status {
	case "healthy":
		fmt.Printf("Vault:       healthy (%s)\n", status.Vault.Mode)
	case "sealed":
		fmt.Println("Vault:       sealed")
		fmt.Println()
		fmt.Println("Vault is sealed and needs to be unsealed.")
		fmt.Println("  Check initialization: vault status")
	case "not_responding":
		fmt.Println("Vault:       not responding")
	default:
		fmt.Printf("Vault:       %s\n", status.Vault.Status)
	}

	// Nomad status
	switch status.Nomad.Status {
	case "healthy":
		fmt.Println("Nomad:       healthy")
	case "not_responding":
		fmt.Println("Nomad:       not responding")
		fmt.Println()
		fmt.Println("Nomad may still be starting up.")
		fmt.Printf("  Check logs:  tail -f %s/nomad.log\n", logDir)
		fmt.Println("  Restart:     styx stop && styx init")
		return nil
	default:
		fmt.Printf("Nomad:       %s\n", status.Nomad.Status)
		fmt.Println()
		fmt.Println("Nomad is responding but reporting unhealthy status.")
		fmt.Printf("  Check logs:  tail -f %s/nomad.log\n", logDir)
		fmt.Println("  Restart:     styx stop && styx init")
		return nil
	}

	// Mode and node info
	fmt.Printf("Mode:        %s\n", status.Mode)
	fmt.Printf("Node Name:   %s\n", status.NodeName)
	fmt.Printf("Datacenter:  %s\n", status.Datacenter)
	fmt.Printf("Region:      %s\n", status.Region)

	// Cluster members (servers only)
	if status.Mode == "server" && len(status.Members) > 0 {
		fmt.Println("\nCluster Members:")
		for _, m := range status.Members {
			fmt.Printf("  - %s (%s:%d) [%s]\n", m.Name, m.Addr, m.Port, m.Status)
		}
	}

	// Known servers (clients only)
	if status.Mode == "client" && status.KnownServers != "" {
		fmt.Printf("\nConnected Nomad Servers: %s\n", status.KnownServers)
	}

	// Core services
	fmt.Println("\nCore Services:")
	fmt.Println("  Nomad UI:    http://127.0.0.1:4646")
	if status.Mode == "server" {
		fmt.Println("  Vault UI:    http://127.0.0.1:8200/ui")
	}

	// Platform endpoints (servers only)
	if status.Mode == "server" {
		fmt.Println("\nPlatform Endpoints:")
		fmt.Println("  Traefik:     http://localhost:4200")
		fmt.Println("  Grafana:     http://localhost:4200/grafana")
		fmt.Println("  Prometheus:  http://localhost:4200/prometheus")
	}

	fmt.Println("\nTransport encryption: Tailscale WireGuard")

	return nil
}
