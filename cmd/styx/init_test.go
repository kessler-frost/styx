package main

import "testing"

// TestValidateJoinIP is the regression test for the missing --join input
// validation. Previously any string was passed straight through to runClient.
func TestValidateJoinIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"valid ipv4", "192.168.1.10", false},
		{"valid ipv4 tailscale", "100.97.142.17", false},
		{"valid ipv6", "fd7a:115c:a1e0::1", false},
		{"empty", "", true},
		{"hostname", "myserver", true},
		{"ip with port", "192.168.1.10:4646", true},
		{"ip with leading space", " 192.168.1.10", true},
		{"ip with trailing space", "192.168.1.10 ", true},
		{"url", "http://192.168.1.10", true},
		{"garbage", "not-an-ip", true},
		{"out of range octet", "999.999.999.999", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJoinIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateJoinIP(%q) error = %v, wantErr = %v", tt.ip, err, tt.wantErr)
			}
		})
	}
}
