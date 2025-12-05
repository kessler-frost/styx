package config

// ServerConfigTemplate is the HCL template for a Nomad server node.
// Server nodes participate in consensus and can also run workloads.
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
}

plugin_dir = "{{.PluginDir}}"

plugin "apple-container" {
  config {
    container_bin_path = "/usr/local/bin/container"
  }
}
`

// ClientConfigTemplate is the HCL template for a Nomad client node.
// Client nodes only run workloads and connect to server(s) for scheduling.
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
}

plugin_dir = "{{.PluginDir}}"

plugin "apple-container" {
  config {
    container_bin_path = "/usr/local/bin/container"
  }
}
`
