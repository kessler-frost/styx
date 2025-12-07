package bootstrap

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// FetchBootstrapFiles fetches all certificates and keys from a server's bootstrap endpoint.
func FetchBootstrapFiles(serverIP, certsDir, secretsDir string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := fmt.Sprintf("http://%s:%d", serverIP, Port)

	// Files to fetch: path -> (local filename, permissions)
	certFiles := []struct {
		endpoint  string
		localName string
		perm      os.FileMode
	}{
		{"/bootstrap/consul-ca.pem", "consul-agent-ca.pem", 0644},
		{"/bootstrap/consul-client-cert.pem", "dc1-client-consul-0.pem", 0644},
		{"/bootstrap/consul-client-key.pem", "dc1-client-consul-0-key.pem", 0600},
		{"/bootstrap/nomad-ca.pem", "nomad-agent-ca.pem", 0644},
		{"/bootstrap/nomad-client-cert.pem", "global-client-nomad.pem", 0644},
		{"/bootstrap/nomad-client-key.pem", "global-client-nomad-key.pem", 0600},
	}

	for _, f := range certFiles {
		data, err := fetchFile(client, baseURL+f.endpoint)
		if err != nil {
			return fmt.Errorf("failed to fetch %s: %w", f.endpoint, err)
		}
		path := filepath.Join(certsDir, f.localName)
		if err := os.WriteFile(path, data, f.perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", f.localName, err)
		}
	}

	// Fetch and save gossip key
	gossipData, err := fetchFile(client, baseURL+"/bootstrap/gossip.key")
	if err != nil {
		return fmt.Errorf("failed to fetch gossip key: %w", err)
	}

	gossipPath := filepath.Join(secretsDir, "gossip.key")
	if err := os.WriteFile(gossipPath, gossipData, 0600); err != nil {
		return fmt.Errorf("failed to write gossip key: %w", err)
	}

	return nil
}

// CheckBootstrapServer checks if a bootstrap server is running at the given IP.
func CheckBootstrapServer(serverIP string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://%s:%d/bootstrap/health", serverIP, Port)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func fetchFile(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
