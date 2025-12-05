package network

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

// TailscaleInfo contains Tailscale network information for this machine.
type TailscaleInfo struct {
	IP       string // IPv4 address (e.g., "100.97.142.17")
	Hostname string // Machine hostname (e.g., "fimbulwinter")
	DNSName  string // Full MagicDNS name (e.g., "fimbulwinter.panthera-frog.ts.net")
	Running  bool   // Whether Tailscale is connected
}

// tailscaleStatus represents the relevant fields from `tailscale status --json`
type tailscaleStatus struct {
	BackendState   string   `json:"BackendState"`
	TailscaleIPs   []string `json:"TailscaleIPs"`
	MagicDNSSuffix string   `json:"MagicDNSSuffix"`
	Self           struct {
		HostName string `json:"HostName"`
		DNSName  string `json:"DNSName"`
	} `json:"Self"`
}

// GetTailscaleInfo returns Tailscale network information for this machine.
// Returns a TailscaleInfo with Running=false if Tailscale is not available.
func GetTailscaleInfo() TailscaleInfo {
	// Find tailscale binary - check common locations
	tailscalePaths := []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/usr/local/bin/tailscale",
		"tailscale",
	}

	var tailscaleBin string
	for _, p := range tailscalePaths {
		if p == "tailscale" {
			// Check PATH
			if path, err := exec.LookPath(p); err == nil {
				tailscaleBin = path
				break
			}
		} else {
			// Check absolute path
			if _, err := exec.LookPath(p); err == nil {
				tailscaleBin = p
				break
			}
			// Also try direct file access for absolute paths
			if abs, _ := filepath.Abs(p); abs != "" {
				if _, err := exec.Command("test", "-x", p).CombinedOutput(); err == nil {
					tailscaleBin = p
					break
				}
			}
		}
	}

	if tailscaleBin == "" {
		return TailscaleInfo{Running: false}
	}

	// Run tailscale status --json
	cmd := exec.Command(tailscaleBin, "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return TailscaleInfo{Running: false}
	}

	var status tailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return TailscaleInfo{Running: false}
	}

	// Check if Tailscale is running
	if status.BackendState != "Running" {
		return TailscaleInfo{Running: false}
	}

	// Extract IPv4 address (first one that looks like IPv4)
	var ipv4 string
	for _, ip := range status.TailscaleIPs {
		if !strings.Contains(ip, ":") {
			ipv4 = ip
			break
		}
	}

	// Clean up DNSName (remove trailing dot)
	dnsName := strings.TrimSuffix(status.Self.DNSName, ".")

	return TailscaleInfo{
		IP:       ipv4,
		Hostname: strings.ToLower(status.Self.HostName),
		DNSName:  strings.ToLower(dnsName),
		Running:  true,
	}
}
