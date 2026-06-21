package container

import (
	"encoding/json"
	"testing"
)

// TestContainerInfoUnmarshal verifies that the JSON tags on ContainerInfo match
// the shape of `container inspect`/`container list --format json` output. The
// driver's IsRunning and exit-classification logic depend on Status parsing
// correctly, so a tag regression here would silently break exit detection.
func TestContainerInfoUnmarshal(t *testing.T) {
	raw := `[{
		"status": "running",
		"networks": [
			{"network": "styx", "address": "192.168.200.5", "gateway": "192.168.200.1", "hostname": "web"}
		],
		"configuration": {
			"id": "web-server",
			"image": {"reference": "docker.io/library/nginx:latest"},
			"resources": {"memoryInBytes": 536870912, "cpus": 2}
		}
	}]`

	var infos []ContainerInfo
	if err := json.Unmarshal([]byte(raw), &infos); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("got %d containers, want 1", len(infos))
	}

	info := infos[0]
	if info.Status != "running" {
		t.Errorf("Status = %q, want running", info.Status)
	}
	if info.Configuration.ID != "web-server" {
		t.Errorf("Configuration.ID = %q, want web-server", info.Configuration.ID)
	}
	if info.Configuration.Resources.CPUs != 2 {
		t.Errorf("Resources.CPUs = %d, want 2", info.Configuration.Resources.CPUs)
	}
	if info.Configuration.Resources.MemoryInBytes != 536870912 {
		t.Errorf("Resources.MemoryInBytes = %d, want 536870912", info.Configuration.Resources.MemoryInBytes)
	}
	if len(info.Networks) != 1 || info.Networks[0].Address != "192.168.200.5" {
		t.Errorf("Networks not parsed correctly: %+v", info.Networks)
	}
}

// TestContainerInfoStatusVariants ensures the various terminal status strings
// round-trip through JSON, matching what classifyExit branches on.
func TestContainerInfoStatusVariants(t *testing.T) {
	for _, status := range []string{"running", "stopped", "exited", "error"} {
		raw := `{"status":"` + status + `"}`
		var info ContainerInfo
		if err := json.Unmarshal([]byte(raw), &info); err != nil {
			t.Fatalf("unmarshal %q failed: %v", status, err)
		}
		if info.Status != status {
			t.Errorf("Status = %q, want %q", info.Status, status)
		}
	}
}

// TestContainerStatsUnmarshal validates the stats JSON tags used by TaskStats.
func TestContainerStatsUnmarshal(t *testing.T) {
	raw := `[{
		"container": "web-server",
		"cpuPercent": 12.5,
		"memoryUsageBytes": 1048576,
		"memoryLimitBytes": 536870912,
		"pids": 7
	}]`

	var stats []ContainerStats
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("got %d stats, want 1", len(stats))
	}
	if stats[0].ContainerID != "web-server" {
		t.Errorf("ContainerID = %q, want web-server", stats[0].ContainerID)
	}
	if stats[0].CPUPercent != 12.5 {
		t.Errorf("CPUPercent = %v, want 12.5", stats[0].CPUPercent)
	}
	if stats[0].PIDs != 7 {
		t.Errorf("PIDs = %d, want 7", stats[0].PIDs)
	}
}
