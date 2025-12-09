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
	Hostname string // Machine hostname (e.g., "myhost")
	DNSName  string // Full MagicDNS name (e.g., "myhost.example-tailnet.ts.net")
	Running  bool   // Whether Tailscale is connected
}

// TailscalePeer represents a peer on the Tailscale network.
type TailscalePeer struct {
	IP       string // IPv4 address
	Hostname string // Machine hostname
	DNSName  string // Full MagicDNS name
	Online   bool   // Whether peer is currently online
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
	Peer map[string]tailscalePeerInfo `json:"Peer"`
}

type tailscalePeerInfo struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

// findTailscaleBinary finds the tailscale binary, preferring PATH lookup.
func findTailscaleBinary() string {
	// First try PATH lookup
	if path, err := exec.LookPath("tailscale"); err == nil {
		return path
	}

	// Fall back to known macOS locations (e.g., Mac App Store install)
	knownPaths := []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
	}

	for _, p := range knownPaths {
		if abs, _ := filepath.Abs(p); abs != "" {
			if _, err := exec.Command("test", "-x", p).CombinedOutput(); err == nil {
				return p
			}
		}
	}
	return ""
}

// getTailscaleStatus runs tailscale status --json and parses the result.
func getTailscaleStatus() *tailscaleStatus {
	tailscaleBin := findTailscaleBinary()
	if tailscaleBin == "" {
		return nil
	}

	cmd := exec.Command(tailscaleBin, "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var status tailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil
	}

	if status.BackendState != "Running" {
		return nil
	}

	return &status
}

// extractIPv4 returns the first IPv4 address from a list of IPs.
func extractIPv4(ips []string) string {
	for _, ip := range ips {
		if !strings.Contains(ip, ":") {
			return ip
		}
	}
	return ""
}

// GetTailscaleInfo returns Tailscale network information for this machine.
// Returns a TailscaleInfo with Running=false if Tailscale is not available.
func GetTailscaleInfo() TailscaleInfo {
	status := getTailscaleStatus()
	if status == nil {
		return TailscaleInfo{Running: false}
	}

	return TailscaleInfo{
		IP:       extractIPv4(status.TailscaleIPs),
		Hostname: strings.ToLower(status.Self.HostName),
		DNSName:  strings.ToLower(strings.TrimSuffix(status.Self.DNSName, ".")),
		Running:  true,
	}
}

// GetTailscalePeers returns all online peers on the Tailscale network.
// Returns nil if Tailscale is not available or not running.
func GetTailscalePeers() []TailscalePeer {
	status := getTailscaleStatus()
	if status == nil {
		return nil
	}

	var peers []TailscalePeer
	for _, peer := range status.Peer {
		if !peer.Online {
			continue
		}

		ipv4 := extractIPv4(peer.TailscaleIPs)
		if ipv4 == "" {
			continue
		}

		peers = append(peers, TailscalePeer{
			IP:       ipv4,
			Hostname: strings.ToLower(peer.HostName),
			DNSName:  strings.ToLower(strings.TrimSuffix(peer.DNSName, ".")),
			Online:   true,
		})
	}

	return peers
}
