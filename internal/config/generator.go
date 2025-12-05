package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// ServerConfig holds the configuration values for a Nomad server node.
type ServerConfig struct {
	DataDir         string // e.g., /var/lib/nomad
	AdvertiseIP     string // Local IP for cluster communication
	BootstrapExpect int    // Number of servers to expect (usually 1 for single node)
	PluginDir       string // Path to task driver plugins
}

// ClientConfig holds the configuration values for a Nomad client node.
type ClientConfig struct {
	DataDir     string   // e.g., /var/lib/nomad
	AdvertiseIP string   // Local IP for cluster communication
	Servers     []string // Server IPs to join
	PluginDir   string   // Path to task driver plugins
}

// GenerateServerConfig renders the server HCL template with the given config.
func GenerateServerConfig(cfg ServerConfig) (string, error) {
	tmpl, err := template.New("server").Parse(ServerConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse server template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to render server config: %w", err)
	}

	return buf.String(), nil
}

// GenerateClientConfig renders the client HCL template with the given config.
func GenerateClientConfig(cfg ClientConfig) (string, error) {
	tmpl, err := template.New("client").Parse(ClientConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse client template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to render client config: %w", err)
	}

	return buf.String(), nil
}

// WriteConfig writes the given content to the specified path.
// It creates parent directories if they don't exist.
func WriteConfig(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}

	return nil
}
