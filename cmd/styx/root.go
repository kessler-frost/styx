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

	// Global directory flags
	dataDir      string
	configDir    string
	pluginDir    string
	logDir       string
	secretsDir   string
	vaultDataDir string
)

var rootCmd = &cobra.Command{
	Use:   "styx",
	Short: "Unite your Macs into a fleet for any workload",
	Long: `Styx unites your Mac devices into a cohesive fleet for running workloads at any scale using Apple Containers and HashiCorp Nomad.

Commands:
  styx init              Start or join a Styx cluster
  styx init --serve      Force server mode
  styx init --join <ip>  Join a specific server
  styx stop              Stop the Styx service
  styx status            Show cluster status
  styx services          Manage platform services`,
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	styxBase := filepath.Join(home, ".styx")

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", filepath.Join(styxBase, "nomad"), "Nomad data directory")
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", filepath.Join(styxBase, "config"), "Config directory")
	rootCmd.PersistentFlags().StringVar(&pluginDir, "plugin-dir", filepath.Join(styxBase, "plugins"), "Plugin directory")
	rootCmd.PersistentFlags().StringVar(&logDir, "log-dir", filepath.Join(styxBase, "logs"), "Log directory")
	rootCmd.PersistentFlags().StringVar(&secretsDir, "secrets-dir", filepath.Join(styxBase, "secrets"), "Secrets directory")
	rootCmd.PersistentFlags().StringVar(&vaultDataDir, "vault-data-dir", filepath.Join(styxBase, "vault"), "Vault data directory")
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
