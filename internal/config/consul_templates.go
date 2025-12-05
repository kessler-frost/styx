package config

// ConsulServerConfigTemplate is the HCL template for a Consul server node.
// Server nodes participate in consensus and provide the service catalog.
const ConsulServerConfigTemplate = `data_dir = "{{.DataDir}}"
bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"
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
  https = 8501
}

connect {
  enabled = true
}

# TLS Configuration
tls {
  defaults {
    ca_file   = "{{.CAFile}}"
    cert_file = "{{.CertFile}}"
    key_file  = "{{.KeyFile}}"
    verify_incoming = true
    verify_outgoing = true
  }
}

# Gossip Encryption
encrypt = "{{.GossipKey}}"

# Auto-encrypt for client certificates
auto_encrypt {
  allow_tls = true
}
`

// ConsulClientConfigTemplate is the HCL template for a Consul client node.
// Client nodes forward requests to servers and cache results locally.
const ConsulClientConfigTemplate = `data_dir = "{{.DataDir}}"
bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"
advertise_addr = "{{.AdvertiseIP}}"
datacenter = "dc1"

server = false
retry_join = [{{range $i, $s := .Servers}}{{if $i}}, {{end}}"{{$s}}"{{end}}]

ports {
  dns = 8600
  http = 8500
  https = 8501
}

connect {
  enabled = true
}

# TLS Configuration
tls {
  defaults {
    ca_file   = "{{.CAFile}}"
    cert_file = "{{.CertFile}}"
    key_file  = "{{.KeyFile}}"
    verify_incoming = true
    verify_outgoing = true
  }
}

# Gossip Encryption
encrypt = "{{.GossipKey}}"
`
