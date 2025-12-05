package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	dataDir   string
	configDir string
	pluginDir string
)

var rootCmd = &cobra.Command{
	Use:   "styx",
	Short: "Distributed system platform for Mac fleets",
	Long: `Styx is a distributed system platform that uses Apple Containers
and HashiCorp Nomad to orchestrate workloads across Mac fleets.

Use 'styx init --server' to start a server node, or 'styx join <ip>'
to join an existing cluster as a client node.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "/var/lib/nomad", "Nomad data directory")
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "/etc/nomad.d", "Nomad config directory")
	rootCmd.PersistentFlags().StringVar(&pluginDir, "plugin-dir", "/usr/local/lib/styx/plugins", "Plugin directory")
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
