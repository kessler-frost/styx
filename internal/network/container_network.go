package network

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	// StyxNetworkName is the name of the container network used by Styx
	StyxNetworkName = "styx"
	// StyxNetworkSubnet is the subnet for the Styx container network
	StyxNetworkSubnet = "192.168.200.0/24"
)

// EnsureStyxNetwork creates the styx network if it doesn't exist
func EnsureStyxNetwork() error {
	// Check if network already exists
	exists, err := networkExists(StyxNetworkName)
	if err != nil {
		return fmt.Errorf("failed to check network: %w", err)
	}

	if exists {
		return nil
	}

	// Create the network
	cmd := exec.Command("/usr/local/bin/container", "network", "create",
		"--subnet", StyxNetworkSubnet,
		StyxNetworkName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create network: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// DeleteStyxNetwork removes the styx network
func DeleteStyxNetwork() error {
	exists, err := networkExists(StyxNetworkName)
	if err != nil {
		return fmt.Errorf("failed to check network: %w", err)
	}

	if !exists {
		return nil
	}

	cmd := exec.Command("/usr/local/bin/container", "network", "delete", StyxNetworkName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete network: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// NetworkExists checks if the styx network exists
func NetworkExists() bool {
	exists, err := networkExists(StyxNetworkName)
	if err != nil {
		// If we can't check, assume it doesn't exist
		return false
	}
	return exists
}

// networkExists checks if a network with the given name exists
func networkExists(name string) (bool, error) {
	cmd := exec.Command("/usr/local/bin/container", "network", "list")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list networks: %w", err)
	}

	// Parse output - format is: NETWORK  STATE  SUBNET
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] == name {
			return true, nil
		}
	}

	return false, nil
}
