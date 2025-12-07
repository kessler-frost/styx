package main

import (
	"fmt"

	"github.com/kessler-frost/styx/internal/services"
	"github.com/spf13/cobra"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage platform services",
	Long: `Manage Styx platform services (NATS, Dragonfly).

Platform services are automatically deployed when starting a server.
Use this command to view status or manually start/stop services.`,
	RunE: runServicesList,
}

var servicesStartCmd = &cobra.Command{
	Use:   "start <service>",
	Short: "Start a platform service",
	Args:  cobra.ExactArgs(1),
	RunE:  runServicesStart,
}

var servicesStopCmd = &cobra.Command{
	Use:   "stop <service>",
	Short: "Stop a platform service",
	Args:  cobra.ExactArgs(1),
	RunE:  runServicesStop,
}

func init() {
	servicesCmd.AddCommand(servicesStartCmd)
	servicesCmd.AddCommand(servicesStopCmd)
	rootCmd.AddCommand(servicesCmd)
}

func runServicesList(cmd *cobra.Command, args []string) error {
	// Check if Nomad is running
	client := services.DefaultClient()
	if !client.IsHealthy() {
		fmt.Println("Nomad is not running. Start Styx first with 'styx init'")
		return nil
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
	fmt.Println("  NATS:      nats://localhost:14222")
	fmt.Println("  Dragonfly: redis://localhost:16379")

	return nil
}

func runServicesStart(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Verify it's a known service
	svc := services.GetService(name)
	if svc == nil {
		return fmt.Errorf("unknown service: %s\n\nAvailable services: nats, dragonfly", name)
	}

	// Check if Nomad is running
	client := services.DefaultClient()
	if !client.IsHealthy() {
		return fmt.Errorf("Nomad is not running. Start Styx first with 'styx init'")
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
		return fmt.Errorf("unknown service: %s\n\nAvailable services: nats, dragonfly", name)
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
