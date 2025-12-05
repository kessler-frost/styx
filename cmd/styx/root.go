package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	dataDir   string
	configDir string
	pluginDir string
	logDir    string
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
	home, _ := os.UserHomeDir()
	styxBase := filepath.Join(home, "Library", "Application Support", "styx")

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", filepath.Join(styxBase, "nomad"), "Nomad data directory")
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", filepath.Join(styxBase, "config"), "Nomad config directory")
	rootCmd.PersistentFlags().StringVar(&pluginDir, "plugin-dir", filepath.Join(styxBase, "plugins"), "Plugin directory")
	rootCmd.PersistentFlags().StringVar(&logDir, "log-dir", filepath.Join(home, "Library", "Logs", "styx"), "Log directory")
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
