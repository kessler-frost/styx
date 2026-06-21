package services

import (
	"strings"
	"testing"
)

func TestIsMandatoryService(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"traefik", true},
		{"nats", false},
		{"prometheus", false},
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := IsMandatoryService(tt.name); got != tt.want {
			t.Errorf("IsMandatoryService(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsOptionalService(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"nats", true},
		{"dragonfly", true},
		{"grafana", true},
		{"rustfs", true},
		{"traefik", false}, // mandatory, not optional
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := IsOptionalService(tt.name); got != tt.want {
			t.Errorf("IsOptionalService(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestServiceClassificationDisjoint ensures no service is both mandatory and
// optional, and that every registered service is classified as one or the other.
func TestServiceClassificationDisjoint(t *testing.T) {
	for _, svc := range PlatformServices {
		mandatory := IsMandatoryService(svc.Name)
		optional := IsOptionalService(svc.Name)
		if mandatory && optional {
			t.Errorf("service %q is both mandatory and optional", svc.Name)
		}
		if !mandatory && !optional {
			t.Errorf("service %q is neither mandatory nor optional", svc.Name)
		}
	}
}

func TestGetService(t *testing.T) {
	svc := GetService("traefik")
	if svc == nil {
		t.Fatal("GetService(traefik) = nil, want service")
	}
	if svc.Name != "traefik" {
		t.Errorf("GetService(traefik).Name = %q, want traefik", svc.Name)
	}

	if GetService("does-not-exist") != nil {
		t.Error("GetService(does-not-exist) = non-nil, want nil")
	}
}

// TestPlatformServicesWellFormed verifies registry invariants: unique names,
// and each service has exactly one HCL source (static string or generator fn).
func TestPlatformServicesWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, svc := range PlatformServices {
		if svc.Name == "" {
			t.Error("found service with empty name")
		}
		if seen[svc.Name] {
			t.Errorf("duplicate service name %q", svc.Name)
		}
		seen[svc.Name] = true

		hasStatic := svc.JobHCL != ""
		hasFunc := svc.JobHCLFunc != nil
		if hasStatic == hasFunc {
			t.Errorf("service %q must have exactly one of JobHCL/JobHCLFunc (static=%v func=%v)",
				svc.Name, hasStatic, hasFunc)
		}
	}
}

func TestTraefikJobHCLSubstitution(t *testing.T) {
	out := TraefikJobHCL("100.64.0.1")
	if strings.Contains(out, "{{NOMAD_ADDR}}") {
		t.Errorf("TraefikJobHCL left placeholder unreplaced:\n%s", out)
	}
	if !strings.Contains(out, "100.64.0.1") {
		t.Errorf("TraefikJobHCL did not substitute address:\n%s", out)
	}
}

func TestPrometheusJobHCLSubstitution(t *testing.T) {
	out := PrometheusJobHCL("10.1.2.3")
	if strings.Contains(out, "{{NOMAD_ADDR}}") {
		t.Errorf("PrometheusJobHCL left placeholder unreplaced")
	}
	if !strings.Contains(out, "10.1.2.3") {
		t.Errorf("PrometheusJobHCL did not substitute address")
	}
}

func TestPromtailJobHCLSubstitution(t *testing.T) {
	out := PromtailJobHCL("/home/user/.styx/nomad/alloc")
	if strings.Contains(out, "{{NOMAD_ALLOC_DIR}}") {
		t.Errorf("PromtailJobHCL left placeholder unreplaced")
	}
	if !strings.Contains(out, "/home/user/.styx/nomad/alloc") {
		t.Errorf("PromtailJobHCL did not substitute alloc dir")
	}
}
