package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kessler-frost/styx/driver/container"
	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/kessler-frost/styx/internal/network"
	"github.com/spf13/cobra"
)

var (
	uninstallYes      bool
	uninstallAll      bool
	uninstallKeepData bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Completely remove styx and its data",
	Long: `Uninstall styx, removing all data, configuration, and optionally dependencies.

This command will:
1. Stop all styx services
2. Remove all styx containers and volumes
3. Remove the styx container network
4. Remove styx data directory (~/.styx)
5. Remove launchd plist
6. Remove styx binaries (if installed to ~/.local)
7. Optionally remove Homebrew-installed dependencies (nomad, vault, container, tailscale)`,
	RunE: runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false, "Skip confirmation prompts")
	uninstallCmd.Flags().BoolVar(&uninstallAll, "all", false, "Also remove all Homebrew-installed dependencies")
	uninstallCmd.Flags().BoolVar(&uninstallKeepData, "keep-data", false, "Keep ~/.styx data directory")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Confirm uninstall
	if !uninstallYes {
		fmt.Print("This will completely remove styx and all its data. Continue? [y/N]: ")
		if !askConfirm() {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
	}

	fmt.Println("Uninstalling styx...")

	// 1. Stop services (ignore errors - may not be running)
	fmt.Println("  Stopping services...")
	_ = runStop(nil, nil)

	// 2. Remove containers, volumes
	fmt.Println("  Removing containers and volumes...")
	removeContainersAndVolumes()

	// 3. Remove container network
	fmt.Println("  Removing container network...")
	if err := network.DeleteStyxNetwork(); err != nil {
		fmt.Printf("    Warning: could not remove network: %v\n", err)
	}

	// 4. Remove data directory
	if !uninstallKeepData {
		styxDir := filepath.Join(home, ".styx")
		fmt.Printf("  Removing %s...\n", styxDir)
		if err := os.RemoveAll(styxDir); err != nil {
			fmt.Printf("    Warning: could not remove data directory: %v\n", err)
		}
	}

	// 5. Remove plist
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.styx.nomad.plist")
	fmt.Printf("  Removing %s...\n", plistPath)
	_ = launchd.Unload(plistPath)
	_ = os.Remove(plistPath)

	// 6. Remove binaries (if installed to ~/.local)
	localBin := filepath.Join(home, ".local", "bin", "styx")
	localLib := filepath.Join(home, ".local", "lib", "styx")
	if _, err := os.Stat(localBin); err == nil {
		fmt.Printf("  Removing %s...\n", localBin)
		_ = os.Remove(localBin)
	}
	if _, err := os.Stat(localLib); err == nil {
		fmt.Printf("  Removing %s...\n", localLib)
		_ = os.RemoveAll(localLib)
	}

	// 7. Handle dependencies
	fmt.Println()
	removeDependencies()

	fmt.Println()
	fmt.Println("Styx uninstalled successfully.")
	return nil
}

func removeContainersAndVolumes() {
	binPath, err := exec.LookPath("container")
	if err != nil {
		// Container CLI not found, nothing to clean up
		return
	}
	client := container.NewClient(binPath)
	ctx := context.Background()

	// Stop and remove all containers
	containers, err := client.List(ctx, true)
	if err == nil {
		for _, c := range containers {
			_ = client.Stop(ctx, c.Configuration.ID)
			_ = client.Remove(ctx, c.Configuration.ID)
		}
	}

	// Remove all volumes
	volumes, err := client.VolumeList(ctx)
	if err == nil {
		for _, v := range volumes {
			_ = client.VolumeRemove(ctx, v.Name)
		}
	}
}

func removeDependencies() {
	deps := []struct {
		name      string
		checkCmd  string
		isCask    bool
		uninstall string
	}{
		{"nomad", "hashicorp/tap/nomad", false, "brew uninstall hashicorp/tap/nomad"},
		{"vault", "hashicorp/tap/vault", false, "brew uninstall hashicorp/tap/vault"},
		{"container", "container", false, "brew uninstall container"},
		{"tailscale", "tailscale-app", true, "brew uninstall --cask tailscale-app"},
	}

	for _, dep := range deps {
		if !isBrewInstalled(dep.checkCmd, dep.isCask) {
			continue
		}

		if uninstallAll {
			fmt.Printf("  Removing %s...\n", dep.name)
			_ = exec.Command("sh", "-c", dep.uninstall).Run()
		} else if !uninstallYes {
			fmt.Printf("Remove %s? (installed via Homebrew) [y/N]: ", dep.name)
			if askConfirm() {
				fmt.Printf("  Removing %s...\n", dep.name)
				_ = exec.Command("sh", "-c", dep.uninstall).Run()
			}
		}
	}
}

func isBrewInstalled(formula string, isCask bool) bool {
	var cmd *exec.Cmd
	if isCask {
		cmd = exec.Command("brew", "list", "--cask", formula)
	} else {
		cmd = exec.Command("brew", "list", formula)
	}
	return cmd.Run() == nil
}

func askConfirm() bool {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}
