package api

import (
	"encoding/json"
	"testing"
)

// TestAgentMembersUnmarshal pins the JSON-tag mapping for Nomad's
// /v1/agent/members response, which getClusterMembers maps into []Member for
// the status command. Nomad uses capitalized keys, so a tag regression would
// silently produce empty member rows.
func TestAgentMembersUnmarshal(t *testing.T) {
	raw := `{
		"Members": [
			{"Name": "mac1.global", "Addr": "10.0.0.1", "Port": 4648, "Status": "alive", "Tags": {"role": "server"}},
			{"Name": "mac2.global", "Addr": "10.0.0.2", "Port": 4648, "Status": "left", "Tags": {"role": "server"}}
		]
	}`

	var members AgentMembers
	if err := json.Unmarshal([]byte(raw), &members); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(members.Members) != 2 {
		t.Fatalf("got %d members, want 2", len(members.Members))
	}

	first := members.Members[0]
	if first.Name != "mac1.global" || first.Addr != "10.0.0.1" || first.Port != 4648 {
		t.Errorf("first member mapped incorrectly: %+v", first)
	}
	if first.Status != "alive" || first.Tags.Role != "server" {
		t.Errorf("first member status/role incorrect: status=%q role=%q", first.Status, first.Tags.Role)
	}
}

// TestAgentMembersEmpty ensures an empty members list parses to a zero-length
// slice (not a panic) — the case the status bounds-handling guards against.
func TestAgentMembersEmpty(t *testing.T) {
	var members AgentMembers
	if err := json.Unmarshal([]byte(`{"Members": []}`), &members); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(members.Members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members.Members))
	}
}

// TestClusterStatusJSONRoundTrip verifies the ClusterStatus struct (emitted by
// `styx status --json`) round-trips, including a nil Members slice.
func TestClusterStatusJSONRoundTrip(t *testing.T) {
	in := ClusterStatus{
		Service:    "running",
		Vault:      VaultStatus{Status: "healthy", Mode: "active"},
		Nomad:      NomadStatus{Status: "healthy"},
		Mode:       "server",
		NodeName:   "mac1",
		Datacenter: "dc1",
		Region:     "global",
		Members:    nil,
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out ClusterStatus
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Service != "running" || out.Mode != "server" || out.NodeName != "mac1" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
	if out.Vault.Status != "healthy" || out.Nomad.Status != "healthy" {
		t.Errorf("nested status round-trip mismatch: %+v", out)
	}
}
