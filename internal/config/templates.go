package config

// ServerConfigTemplate is the HCL template for a Nomad server node.
// Server nodes participate in consensus and can also run workloads.
// Transport encryption is handled by Tailscale (no TLS/Consul needed).
const ServerConfigTemplate = `data_dir  = "{{.DataDir}}"
bind_addr = "0.0.0.0"

advertise {
  http = "{{.AdvertiseIP}}"
  rpc  = "{{.AdvertiseIP}}"
  serf = "{{.AdvertiseIP}}:5648"
}

server {
  enabled          = true
  bootstrap_expect = {{.BootstrapExpect}}
}

client {
  enabled = true

  # Override CPU fingerprinting (apple-container driver doesn't report resources correctly)
  cpu_total_compute = {{.CPUTotalCompute}}
}

plugin_dir = "{{.PluginDir}}"

plugin "apple-container" {
  config {
    container_bin_path = "/usr/local/bin/container"
  }
}

# Vault Integration with Workload Identity
vault {
  enabled = true
  address = "http://127.0.0.1:8200"

  # Workload identity configuration for Nomad 1.7+
  default_identity {
    aud  = ["vault.io"]
    env  = false
    file = true
    ttl  = "1h"
  }
}
`

// ClientConfigTemplate is the HCL template for a Nomad client node.
// Client nodes only run workloads and connect to server(s) for scheduling.
// Transport encryption is handled by Tailscale (no TLS/Consul needed).
const ClientConfigTemplate = `data_dir  = "{{.DataDir}}"
bind_addr = "0.0.0.0"

advertise {
  http = "{{.AdvertiseIP}}"
  rpc  = "{{.AdvertiseIP}}"
  serf = "{{.AdvertiseIP}}:5648"
}

client {
  enabled = true
  servers = [{{range $i, $s := .Servers}}{{if $i}}, {{end}}"{{$s}}:4647"{{end}}]

  # Override CPU fingerprinting (apple-container driver doesn't report resources correctly)
  cpu_total_compute = {{.CPUTotalCompute}}
}

plugin_dir = "{{.PluginDir}}"

plugin "apple-container" {
  config {
    container_bin_path = "/usr/local/bin/container"
  }
}
`
