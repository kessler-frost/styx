package config

// ConsulServerConfigTemplate is the HCL template for a Consul server node.
// Server nodes participate in consensus and provide the service catalog.
const ConsulServerConfigTemplate = `data_dir = "{{.DataDir}}"
bind_addr = "0.0.0.0"
advertise_addr = "{{.AdvertiseIP}}"
datacenter = "dc1"

server = true
bootstrap_expect = {{.BootstrapExpect}}

ui_config {
  enabled = true
}

ports {
  dns = 8600
  http = 8500
}

connect {
  enabled = true
}
`

// ConsulClientConfigTemplate is the HCL template for a Consul client node.
// Client nodes forward requests to servers and cache results locally.
const ConsulClientConfigTemplate = `data_dir = "{{.DataDir}}"
bind_addr = "0.0.0.0"
advertise_addr = "{{.AdvertiseIP}}"
datacenter = "dc1"

server = false
retry_join = [{{range $i, $s := .Servers}}{{if $i}}, {{end}}"{{$s}}"{{end}}]

ports {
  dns = 8600
  http = 8500
}

connect {
  enabled = true
}
`
