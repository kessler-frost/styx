package main

import (
	"strings"
	"testing"

	"github.com/kessler-frost/styx/internal/services"
)

func TestHumanizeBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"one KB", 1024, "1.0 KB"},
		{"kilobytes", 1536, "1.5 KB"},
		{"one MB", 1024 * 1024, "1.0 MB"},
		{"megabytes", 5 * 1024 * 1024, "5.0 MB"},
		{"one GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"gigabytes", 3 * 1024 * 1024 * 1024, "3.0 GB"},
		{"just under KB", 1023, "1023 B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanizeBytes(tt.bytes); got != tt.want {
				t.Errorf("humanizeBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"running", "[running]"},
		{"pending", "[pending]"},
		{"dead", "[stopped]"},
		{"not_deployed", "[not deployed]"},
		{"error", "[error]"},
		{"weird", "[weird]"}, // default echoes the unknown status
	}
	for _, tt := range tests {
		if got := getStatusIcon(tt.status); got != tt.want {
			t.Errorf("getStatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// TestGetAvailableServiceNames verifies the helper joins every registered
// platform service name, so the CLI help text stays in sync with the registry.
func TestGetAvailableServiceNames(t *testing.T) {
	got := getAvailableServiceNames()
	for _, svc := range services.PlatformServices {
		if !strings.Contains(got, svc.Name) {
			t.Errorf("getAvailableServiceNames() = %q, missing %q", got, svc.Name)
		}
	}
	if want := len(services.PlatformServices) - 1; strings.Count(got, ", ") != want {
		t.Errorf("expected %d separators, got %d in %q", want, strings.Count(got, ", "), got)
	}
}
