package main

import (
	"testing"

	"github.com/kessler-frost/styx/internal/api"
)

// TestPrintStatusHumanNoPanic exercises printStatusHuman across a range of
// ClusterStatus shapes — including a server with nil/empty Members and a
// member with zero-value fields — to confirm there are no nil/bounds panics
// in the Members-access path.
func TestPrintStatusHumanNoPanic(t *testing.T) {
	tests := []struct {
		name   string
		status api.ClusterStatus
	}{
		{
			name:   "stopped service",
			status: api.ClusterStatus{Service: "stopped"},
		},
		{
			name: "vault sealed",
			status: api.ClusterStatus{
				Service: "running",
				Vault:   api.VaultStatus{Status: "sealed"},
				Nomad:   api.NomadStatus{Status: "healthy"},
			},
		},
		{
			name: "nomad not responding",
			status: api.ClusterStatus{
				Service: "running",
				Vault:   api.VaultStatus{Status: "healthy", Mode: "active"},
				Nomad:   api.NomadStatus{Status: "not_responding"},
			},
		},
		{
			name: "server with nil members",
			status: api.ClusterStatus{
				Service: "running",
				Vault:   api.VaultStatus{Status: "healthy", Mode: "active"},
				Nomad:   api.NomadStatus{Status: "healthy"},
				Mode:    "server",
				Members: nil,
			},
		},
		{
			name: "server with empty members slice",
			status: api.ClusterStatus{
				Service: "running",
				Vault:   api.VaultStatus{Status: "healthy", Mode: "active"},
				Nomad:   api.NomadStatus{Status: "healthy"},
				Mode:    "server",
				Members: []api.Member{},
			},
		},
		{
			name: "server with members including zero-value member",
			status: api.ClusterStatus{
				Service: "running",
				Vault:   api.VaultStatus{Status: "healthy", Mode: "active"},
				Nomad:   api.NomadStatus{Status: "healthy"},
				Mode:    "server",
				Members: []api.Member{
					{Name: "node1", Addr: "10.0.0.1", Port: 4648, Status: "alive"},
					{}, // zero-value member must not panic
				},
			},
		},
		{
			name: "client with known servers",
			status: api.ClusterStatus{
				Service:      "running",
				Vault:        api.VaultStatus{Status: "not_responding"},
				Nomad:        api.NomadStatus{Status: "healthy"},
				Mode:         "client",
				KnownServers: "10.0.0.1:4647",
			},
		},
		{
			name: "client with empty known servers",
			status: api.ClusterStatus{
				Service: "running",
				Nomad:   api.NomadStatus{Status: "healthy"},
				Mode:    "client",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := printStatusHuman(tt.status); err != nil {
				t.Errorf("printStatusHuman returned error: %v", err)
			}
		})
	}
}
