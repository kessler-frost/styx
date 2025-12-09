package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/kessler-frost/styx/internal/network"
	"github.com/kessler-frost/styx/internal/services"
	"github.com/spf13/cobra"
)

var servicesJSON bool
var startAll bool

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage platform services",
	Long: `Manage Styx platform services.

Mandatory Services (always running):
  traefik    - Ingress controller (Traefik)

Optional Services:
  nats       - Message queue (NATS)
  dragonfly  - Redis-compatible cache (Dragonfly)
  postgres   - PostgreSQL database
  rustfs     - S3-compatible storage (RustFS)
  prometheus - Metrics server
  loki       - Log aggregation
  grafana    - Dashboards
  promtail   - Log shipper

Use 'styx services' to see status of all services.
Use 'styx services start --all' to start all optional services.`,
	RunE: runServicesList,
}

var servicesStartCmd = &cobra.Command{
	Use:   "start [service]",
	Short: "Start a platform service or all optional services",
	Long: `Start a platform service by name, or use --all to start all optional services.

Examples:
  styx services start nats       # Start NATS
  styx services start --all      # Start all optional services
  styx services start -a         # Same as --all`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runServicesStart,
}

var servicesStopCmd = &cobra.Command{
	Use:   "stop <service>",
	Short: "Stop a platform service",
	Args:  cobra.ExactArgs(1),
	RunE:  runServicesStop,
}

func init() {
	servicesCmd.Flags().BoolVar(&servicesJSON, "json", false, "Output in JSON format")
	servicesStartCmd.Flags().BoolVarP(&startAll, "all", "a", false, "Start all optional services")
	servicesCmd.AddCommand(servicesStartCmd)
	servicesCmd.AddCommand(servicesStopCmd)
	rootCmd.AddCommand(servicesCmd)
}

// getAvailableServiceNames returns a comma-separated list of available platform services
func getAvailableServiceNames() string {
	names := make([]string, len(services.PlatformServices))
	for i, svc := range services.PlatformServices {
		names[i] = svc.Name
	}
	return strings.Join(names, ", ")
}

func runServicesList(cmd *cobra.Command, args []string) error {
	// Check if Nomad is running
	svcClient := services.DefaultClient()
	if !svcClient.IsHealthy() {
		if servicesJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("Nomad is not running. Start Styx first with 'styx init'")
		return nil
	}

	// Use API client for JSON output
	if servicesJSON {
		apiClient := api.NewClient()
		svcs, err := apiClient.GetPlatformServices()
		if err != nil {
			return fmt.Errorf("failed to get service status: %w", err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(svcs)
	}

	statuses, err := services.Status()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	fmt.Println("Platform Services")
	fmt.Println("-----------------")
	fmt.Println()

	for _, s := range statuses {
		statusIcon := getStatusIcon(s.Status)
		fmt.Printf("  %s %-12s %s\n", statusIcon, s.Name, s.Description)
	}

	fmt.Println()
	fmt.Println("Endpoints (when running):")
	fmt.Println("  NATS:       nats://localhost:4222")
	fmt.Println("  Dragonfly:  redis://localhost:6379")
	tsInfo := network.GetTailscaleInfo()
	if tsInfo.Running {
		fmt.Printf("  Traefik:    https://%s (ingress)\n", tsInfo.DNSName)
		fmt.Printf("  Grafana:    https://%s/grafana\n", tsInfo.DNSName)
		fmt.Printf("  Prometheus: https://%s/prometheus\n", tsInfo.DNSName)
	} else {
		fmt.Println("  Traefik:    http://localhost:4200 (ingress)")
		fmt.Println("  Grafana:    http://localhost:4200/grafana")
		fmt.Println("  Prometheus: http://localhost:4200/prometheus")
	}
	fmt.Println("              http://localhost:4201 (traefik dashboard)")
	fmt.Println("  Loki:       http://localhost:3100 (internal)")

	return nil
}

func runServicesStart(cmd *cobra.Command, args []string) error {
	// Check if Nomad is running
	client := services.DefaultClient()
	if !client.IsHealthy() {
		return fmt.Errorf("Nomad is not running. Start Styx first with 'styx init'")
	}

	// Handle --all flag
	if startAll {
		if len(args) > 0 {
			return fmt.Errorf("cannot specify both --all and a service name")
		}
		fmt.Println("Starting all optional services...")
		if err := services.DeployOptional(); err != nil {
			return fmt.Errorf("failed to start optional services: %w", err)
		}
		fmt.Println("All optional services started successfully")
		return nil
	}

	// If no args and no --all flag, show helpful message
	if len(args) == 0 {
		return fmt.Errorf("specify a service name or use --all to start all optional services\n\nAvailable services: %s", getAvailableServiceNames())
	}

	name := args[0]

	// Verify it's a known service
	svc := services.GetService(name)
	if svc == nil {
		return fmt.Errorf("unknown service: %s\n\nAvailable services: %s", name, getAvailableServiceNames())
	}

	// Check if it's a mandatory service
	if services.IsMandatoryService(name) {
		return fmt.Errorf("%s is a mandatory service and is already running (started automatically with 'styx init')", name)
	}

	fmt.Printf("Starting %s...\n", name)
	if err := services.Deploy(name); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	fmt.Printf("%s started successfully\n", name)
	return nil
}

func runServicesStop(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Verify it's a known service
	svc := services.GetService(name)
	if svc == nil {
		return fmt.Errorf("unknown service: %s\n\nAvailable services: %s", name, getAvailableServiceNames())
	}

	// Check if Nomad is running
	client := services.DefaultClient()
	if !client.IsHealthy() {
		return fmt.Errorf("Nomad is not running. Start Styx first with 'styx init'")
	}

	fmt.Printf("Stopping %s...\n", name)
	if err := services.Stop(name); err != nil {
		return fmt.Errorf("failed to stop %s: %w", name, err)
	}

	fmt.Printf("%s stopped successfully\n", name)
	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "running":
		return "[running]"
	case "pending":
		return "[pending]"
	case "dead":
		return "[stopped]"
	case "not_deployed":
		return "[not deployed]"
	case "error":
		return "[error]"
	default:
		return "[" + status + "]"
	}
}
