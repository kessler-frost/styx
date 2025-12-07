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

	// Root command flags
	autoYes bool
)

var rootCmd = &cobra.Command{
	Use:   "styx [server-ip]",
	Short: "Unite your Macs into a fleet for any workload",
	Long: `Styx unites your Mac devices into a cohesive fleet for running workloads at any scale using Apple Containers and HashiCorp Nomad.

  styx              Auto-discover and join a server on your Tailscale network
  styx -y           Auto-start as server if no servers found (skip prompt)
  styx <ip>         Join a server at the specified IP address
  styx stop         Stop the Styx service`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStyx,
}

func runStyx(cmd *cobra.Command, args []string) error {
	// If IP provided, join that server
	if len(args) == 1 {
		return runJoin(cmd, args)
	}

	// Otherwise auto-discover
	return runAutoDiscover(cmd, args)
}

func init() {
	home, _ := os.UserHomeDir()
	styxBase := filepath.Join(home, ".styx")

	rootCmd.Flags().BoolVarP(&autoYes, "yes", "y", false, "Auto-confirm prompts (start server if none found)")
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
