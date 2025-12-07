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
	dataDir      string
	configDir    string
	pluginDir    string
	logDir       string
	secretsDir   string
	vaultDataDir string
)

var rootCmd = &cobra.Command{
	Use:   "styx",
	Short: "Distributed system platform for Mac fleets",
	Long: `Styx is a distributed system platform that uses Apple Containers
and HashiCorp Nomad to orchestrate workloads across Mac fleets.

Running 'styx' with no arguments auto-discovers servers on your
Tailscale network and joins one.

Manual options:
  styx init --server   Start a server node
  styx join <ip>       Join an existing cluster`,
	RunE: runAutoDiscover,
}

func init() {
	home, _ := os.UserHomeDir()
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
