package network

import (
	"encoding/json"
	"testing"
)

func TestExtractIPv4(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		want string
	}{
		{"ipv4 only", []string{"100.97.142.17"}, "100.97.142.17"},
		{"ipv4 before ipv6", []string{"100.97.142.17", "fd7a:115c:a1e0::1"}, "100.97.142.17"},
		{"ipv6 before ipv4", []string{"fd7a:115c:a1e0::1", "100.97.142.17"}, "100.97.142.17"},
		{"ipv6 only", []string{"fd7a:115c:a1e0::1"}, ""},
		{"empty", []string{}, ""},
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractIPv4(tt.ips); got != tt.want {
				t.Errorf("extractIPv4(%v) = %q, want %q", tt.ips, got, tt.want)
			}
		})
	}
}

// TestTailscaleStatusUnmarshal verifies the JSON parsing of tailscale status
// output into the internal struct, including IP/DNS extraction logic that
// GetTailscaleInfo relies on.
func TestTailscaleStatusUnmarshal(t *testing.T) {
	raw := `{
		"BackendState": "Running",
		"TailscaleIPs": ["100.97.142.17", "fd7a:115c:a1e0::abcd"],
		"MagicDNSSuffix": "example-tailnet.ts.net",
		"Self": {
			"HostName": "MyHost",
			"DNSName": "myhost.example-tailnet.ts.net."
		},
		"Peer": {
			"node1": {
				"HostName": "Peer-One",
				"DNSName": "peer-one.example-tailnet.ts.net.",
				"TailscaleIPs": ["100.64.0.2"],
				"Online": true
			},
			"node2": {
				"HostName": "Offline-Node",
				"DNSName": "offline.example-tailnet.ts.net.",
				"TailscaleIPs": ["100.64.0.3"],
				"Online": false
			}
		}
	}`

	var status tailscaleStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if status.BackendState != "Running" {
		t.Errorf("BackendState = %q, want Running", status.BackendState)
	}
	if got := extractIPv4(status.TailscaleIPs); got != "100.97.142.17" {
		t.Errorf("self IPv4 = %q, want 100.97.142.17", got)
	}
	if status.Self.HostName != "MyHost" {
		t.Errorf("Self.HostName = %q, want MyHost", status.Self.HostName)
	}
	if len(status.Peer) != 2 {
		t.Fatalf("got %d peers, want 2", len(status.Peer))
	}

	// Mirror the online-filter + IPv4 extraction that GetTailscalePeers performs.
	online := 0
	for _, p := range status.Peer {
		if p.Online && extractIPv4(p.TailscaleIPs) != "" {
			online++
		}
	}
	if online != 1 {
		t.Errorf("got %d online peers with IPv4, want 1", online)
	}
}
