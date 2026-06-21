package config

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetCPUTotalCompute(t *testing.T) {
	got := GetCPUTotalCompute()
	want := runtime.NumCPU() * 1000
	if got != want {
		t.Errorf("GetCPUTotalCompute() = %d, want %d", got, want)
	}
	if got <= 0 {
		t.Errorf("GetCPUTotalCompute() = %d, must be positive", got)
	}
}

func TestGenerateServerConfig(t *testing.T) {
	cfg := ServerConfig{
		DataDir:          "/home/user/.styx/nomad",
		AdvertiseIP:      "192.168.1.50",
		BootstrapExpect:  1,
		PluginDir:        "/home/user/.styx/plugins",
		CPUTotalCompute:  8000,
		ContainerBinPath: "/usr/local/bin/container",
	}

	out, err := GenerateServerConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateServerConfig returned error: %v", err)
	}

	mustContain := []string{
		`data_dir  = "/home/user/.styx/nomad"`,
		`http = "192.168.1.50"`,
		`serf = "192.168.1.50:5648"`,
		`bootstrap_expect = 1`,
		`cpu_total_compute = 8000`,
		`plugin_dir = "/home/user/.styx/plugins"`,
		`container_bin_path = "/usr/local/bin/container"`,
		`enabled = true`,        // server block
		`node_class = "server"`, // server runs workloads too
		`vault {`,               // server enables Vault integration
	}
	for _, frag := range mustContain {
		if !strings.Contains(out, frag) {
			t.Errorf("server config missing %q\n---\n%s", frag, out)
		}
	}

	// Unrendered template directives would indicate a struct/template mismatch.
	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Errorf("server config has unrendered template directives:\n%s", out)
	}
}

func TestGenerateClientConfig(t *testing.T) {
	cfg := ClientConfig{
		DataDir:          "/data",
		AdvertiseIP:      "10.0.0.2",
		Servers:          []string{"10.0.0.1", "10.0.0.3"},
		PluginDir:        "/plugins",
		CPUTotalCompute:  4000,
		ContainerBinPath: "/bin/container",
	}

	out, err := GenerateClientConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateClientConfig returned error: %v", err)
	}

	// Multiple servers must be rendered comma-separated with :4647 RPC port.
	if !strings.Contains(out, `servers = ["10.0.0.1:4647", "10.0.0.3:4647"]`) {
		t.Errorf("client servers not rendered correctly:\n%s", out)
	}
	// Client config must NOT contain a Vault block (server-only).
	if strings.Contains(out, "vault {") {
		t.Errorf("client config should not contain a vault block:\n%s", out)
	}
	if strings.Contains(out, "{{") {
		t.Errorf("client config has unrendered template directives:\n%s", out)
	}
}

func TestGenerateClientConfigSingleServer(t *testing.T) {
	out, err := GenerateClientConfig(ClientConfig{
		AdvertiseIP: "10.0.0.2",
		Servers:     []string{"10.0.0.1"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, `servers = ["10.0.0.1:4647"]`) {
		t.Errorf("single server not rendered correctly:\n%s", out)
	}
}

func TestGenerateVaultConfig(t *testing.T) {
	out, err := GenerateVaultConfig(VaultConfig{
		DataDir:     "/vault/data",
		NodeID:      "mynode",
		AdvertiseIP: "172.16.0.5",
	})
	if err != nil {
		t.Fatalf("GenerateVaultConfig returned error: %v", err)
	}

	mustContain := []string{
		`storage "raft"`,
		`path    = "/vault/data"`,
		`node_id = "mynode"`,
		`api_addr = "http://172.16.0.5:8200"`,
		`cluster_addr = "https://172.16.0.5:8201"`,
		`tls_disable = true`,
	}
	for _, frag := range mustContain {
		if !strings.Contains(out, frag) {
			t.Errorf("vault config missing %q\n---\n%s", frag, out)
		}
	}
	if strings.Contains(out, "{{") {
		t.Errorf("vault config has unrendered template directives:\n%s", out)
	}
}
