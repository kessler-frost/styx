package tailserve

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// findTailscaleBinary finds the tailscale binary in common locations.
func findTailscaleBinary() string {
	tailscalePaths := []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/usr/local/bin/tailscale",
		"tailscale",
	}

	for _, p := range tailscalePaths {
		if p == "tailscale" {
			if path, err := exec.LookPath(p); err == nil {
				return path
			}
			continue
		}
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
		if abs, _ := filepath.Abs(p); abs != "" {
			if _, err := exec.Command("test", "-x", p).CombinedOutput(); err == nil {
				return p
			}
		}
	}
	return ""
}

// Enable sets up Tailscale Serve to forward HTTPS:443 to Traefik at localhost:4200.
// This provides automatic TLS termination via Tailscale.
func Enable() error {
	bin := findTailscaleBinary()
	if bin == "" {
		return fmt.Errorf("tailscale binary not found")
	}

	// Run: tailscale serve --bg localhost:4200
	cmd := exec.Command(bin, "serve", "--bg", "localhost:4200")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable tailscale serve: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Disable stops Tailscale Serve.
func Disable() error {
	bin := findTailscaleBinary()
	if bin == "" {
		return fmt.Errorf("tailscale binary not found")
	}

	// Run: tailscale serve off
	cmd := exec.Command(bin, "serve", "off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disable tailscale serve: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Status returns whether Tailscale Serve is currently active.
func Status() (bool, error) {
	bin := findTailscaleBinary()
	if bin == "" {
		return false, fmt.Errorf("tailscale binary not found")
	}

	// Run: tailscale serve status
	cmd := exec.Command(bin, "serve", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// "tailscale serve status" exits non-zero when nothing is being served
		return false, nil
	}

	// Check if output indicates active serving
	outputStr := strings.TrimSpace(string(output))
	return len(outputStr) > 0 && !strings.Contains(outputStr, "No web serve"), nil
}
