package network

import (
	"strings"
	"testing"
)

// TestHasNomadMembers covers the response-body classification that
// isNomadServer relies on to decide whether a peer is a Nomad server.
func TestHasNomadMembers(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"server with members", `{"Members":[{"Name":"node1.global"}]}`, true},
		{"multiple members", `{"Members":[{"Name":"a"},{"Name":"b"}]}`, true},
		{"empty members", `{"Members":[]}`, false},
		{"null members", `{"Members":null}`, false},
		{"missing members key", `{"Other":"value"}`, false},
		{"malformed json", `not json`, false},
		{"empty body", ``, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasNomadMembers(strings.NewReader(tt.body)); got != tt.want {
				t.Errorf("hasNomadMembers(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}
