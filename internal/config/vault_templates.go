package config

import (
	"bytes"
	"text/template"
)

// VaultServerConfigTemplate is the HCL template for a Vault server.
// Vault uses integrated Raft storage (no external dependencies).
const VaultServerConfigTemplate = `storage "raft" {
  path    = "{{.DataDir}}"
  node_id = "{{.NodeID}}"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

api_addr = "http://{{.AdvertiseIP}}:8200"
cluster_addr = "https://{{.AdvertiseIP}}:8201"

ui = true
disable_mlock = true
`

// VaultConfig holds the configuration values for a Vault server.
type VaultConfig struct {
	DataDir     string // Path to Raft storage directory
	NodeID      string // Unique node identifier
	AdvertiseIP string // Local IP for cluster communication
}

// GenerateVaultConfig renders the Vault HCL template with the given config.
func GenerateVaultConfig(cfg VaultConfig) (string, error) {
	return executeTemplate("vault", VaultServerConfigTemplate, cfg)
}

// executeTemplate is a helper to render any template
func executeTemplate(name, tmpl string, data interface{}) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
