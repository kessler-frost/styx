package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/kessler-frost/styx/internal/network"
	"github.com/spf13/cobra"
)

func runAutoDiscover(cmd *cobra.Command, args []string) error {
	// Check if already running
	if launchd.IsLoaded("com.styx.nomad") {
		fmt.Println("Styx is already running.")
		fmt.Println("Use 'styx status' to check cluster status")
		fmt.Println("Use 'styx stop' to stop the service")
		return nil
	}

	// Check Tailscale status
	tsInfo := network.GetTailscaleInfo()
	if !tsInfo.Running {
		fmt.Println("Tailscale is not running.")
		fmt.Println()
		fmt.Println("To auto-discover servers, install and connect Tailscale:")
		fmt.Println("  https://tailscale.com/download")
		fmt.Println()
		fmt.Println("Or use manual commands:")
		fmt.Println("  styx -y         Start a server on this machine")
		fmt.Println("  styx <ip>       Join an existing server")
		return nil
	}

	fmt.Printf("Tailscale connected: %s (%s)\n", tsInfo.DNSName, tsInfo.IP)
	fmt.Println("Discovering Nomad servers on Tailscale network...")

	servers := network.DiscoverNomadServers(3 * time.Second)

	// No servers found - prompt to start one (or auto-start with -y)
	if len(servers) == 0 {
		fmt.Println()
		fmt.Println("No Nomad servers found on your Tailscale network.")
		fmt.Println()

		if autoYes || promptYesNo("Start a server on this machine?") {
			return runInit(nil, nil)
		}

		fmt.Println()
		fmt.Println("Run 'styx -y' to start a server on this machine.")
		return nil
	}

	// Single server found - auto-join
	if len(servers) == 1 {
		server := servers[0]
		fmt.Printf("\nFound server: %s (%s)\n", server.Hostname, server.IP)
		fmt.Println("Joining cluster...")
		fmt.Println()
		return runJoin(nil, []string{server.IP})
	}

	// Multiple servers found - prompt for selection
	fmt.Printf("\nFound %d Nomad servers:\n", len(servers))
	fmt.Println()
	for i, s := range servers {
		fmt.Printf("  [%d] %s (%s)\n", i+1, s.Hostname, s.IP)
	}
	fmt.Println()

	selected := promptServerSelection(len(servers))
	if selected < 0 {
		return nil
	}

	server := servers[selected]
	fmt.Println("Joining cluster...")
	fmt.Println()
	return runJoin(nil, []string{server.IP})
}

func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [y/N]: ", question)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func promptServerSelection(count int) int {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Select a server [1-%d] or 'q' to quit: ", count)

	input, err := reader.ReadString('\n')
	if err != nil {
		return -1
	}

	input = strings.TrimSpace(input)

	if input == "q" || input == "Q" || input == "" {
		return -1
	}

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > count {
		fmt.Println("Invalid selection")
		return -1
	}

	return num - 1
}
