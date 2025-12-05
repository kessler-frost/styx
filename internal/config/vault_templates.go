package config

import (
	"bytes"
	"text/template"
)

// VaultServerConfigTemplate is the HCL template for a Vault server.
// Vault uses Consul as its storage backend for HA.
const VaultServerConfigTemplate = `storage "consul" {
  address = "127.0.0.1:8500"
  path    = "vault/"
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
