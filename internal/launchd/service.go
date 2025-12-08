package launchd

import (
	"fmt"
	"os/exec"
	"strings"
)

// Load loads a launchd plist file, starting the service.
// Equivalent to: launchctl load <plistPath>
func Load(plistPath string) error {
	cmd := exec.Command("launchctl", "load", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load plist %s: %w\nOutput: %s", plistPath, err, string(output))
	}
	return nil
}

// Unload unloads a launchd plist file, stopping the service.
// Equivalent to: launchctl unload <plistPath>
func Unload(plistPath string) error {
	cmd := exec.Command("launchctl", "unload", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to unload plist %s: %w\nOutput: %s", plistPath, err, string(output))
	}
	return nil
}

// IsLoaded checks if a service with the given label is currently loaded.
func IsLoaded(label string) bool {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), label)
}

// Stop stops a running service by its label.
// Equivalent to: launchctl stop <label>
func Stop(label string) error {
	cmd := exec.Command("launchctl", "stop", label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop service %s: %w\nOutput: %s", label, err, string(output))
	}
	return nil
}

// Start starts a stopped service by its label.
// Equivalent to: launchctl start <label>
func Start(label string) error {
	cmd := exec.Command("launchctl", "start", label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w\nOutput: %s", label, err, string(output))
	}
	return nil
}

// Restart restarts a service by stopping and starting it.
func Restart(label string) error {
	// Stop ignores errors since service might not be running
	if err := Stop(label); err != nil {
		// Log but continue - service might not have been running
		fmt.Printf("Note: service %s was not running or failed to stop: %v\n", label, err)
	}
	return Start(label)
}
