package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kessler-frost/styx/internal/bootstrap"
	"github.com/kessler-frost/styx/internal/network"
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:    "bootstrap-server",
	Short:  "Run the bootstrap server for client discovery",
	Hidden: true, // Internal command, not shown in help
	RunE:   runBootstrapServer,
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}

func runBootstrapServer(cmd *cobra.Command, args []string) error {
	// Get Tailscale IP for binding
	tsInfo := network.GetTailscaleInfo()
	if !tsInfo.Running {
		return fmt.Errorf("Tailscale is not running - bootstrap server requires Tailscale")
	}

	// Start bootstrap server
	server, err := bootstrap.NewServer(tsInfo.IP, certsDir, secretsDir)
	if err != nil {
		return fmt.Errorf("failed to start bootstrap server: %w", err)
	}

	server.Start()
	fmt.Printf("Bootstrap server listening on %s\n", server.Addr())

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down bootstrap server...")
	return server.Stop()
}
