package constants

import "testing"

// TestPortValues pins the documented platform port assignments. These ports
// are referenced across config templates, status output, and the README, so a
// silent change here would desync the whole platform.
func TestPortValues(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"NomadPort", NomadPort, 4646},
		{"VaultPort", VaultPort, 8200},
		{"TraefikHTTPPort", TraefikHTTPPort, 4200},
		{"TraefikDashboardPort", TraefikDashboardPort, 4201},
		{"TraefikMetricsPort", TraefikMetricsPort, 8082},
		{"NATSClientPort", NATSClientPort, 4222},
		{"NATSClusterPort", NATSClusterPort, 6222},
		{"NATSMonitorPort", NATSMonitorPort, 8222},
		{"DragonflyPort", DragonflyPort, 6379},
		{"PrometheusPort", PrometheusPort, 9090},
		{"LokiPort", LokiPort, 3100},
		{"GrafanaPort", GrafanaPort, 3000},
		{"PromtailPort", PromtailPort, 9080},
		{"PostgresPort", PostgresPort, 5432},
		{"RustFSAPIPort", RustFSAPIPort, 9000},
		{"RustFSConsolePort", RustFSConsolePort, 9001},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

// TestPortsUnique guards against two services accidentally sharing a port,
// which would cause a bind conflict at runtime.
func TestPortsUnique(t *testing.T) {
	ports := map[int]string{}
	all := map[string]int{
		"NomadPort":            NomadPort,
		"VaultPort":            VaultPort,
		"TraefikHTTPPort":      TraefikHTTPPort,
		"TraefikDashboardPort": TraefikDashboardPort,
		"TraefikMetricsPort":   TraefikMetricsPort,
		"NATSClientPort":       NATSClientPort,
		"NATSClusterPort":      NATSClusterPort,
		"NATSMonitorPort":      NATSMonitorPort,
		"DragonflyPort":        DragonflyPort,
		"PrometheusPort":       PrometheusPort,
		"LokiPort":             LokiPort,
		"GrafanaPort":          GrafanaPort,
		"PromtailPort":         PromtailPort,
		"PostgresPort":         PostgresPort,
		"RustFSAPIPort":        RustFSAPIPort,
		"RustFSConsolePort":    RustFSConsolePort,
	}

	for name, port := range all {
		if port < 1 || port > 65535 {
			t.Errorf("%s = %d is outside the valid port range", name, port)
		}
		if other, dup := ports[port]; dup {
			t.Errorf("port %d used by both %s and %s", port, other, name)
		}
		ports[port] = name
	}
}
