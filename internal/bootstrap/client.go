package bootstrap

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// FetchBootstrapFiles fetches CA cert and gossip key from a server's bootstrap endpoint.
func FetchBootstrapFiles(serverIP, certsDir, secretsDir string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := fmt.Sprintf("http://%s:%d", serverIP, Port)

	// Fetch and save Consul CA
	caData, err := fetchFile(client, baseURL+"/bootstrap/consul-ca.pem")
	if err != nil {
		return fmt.Errorf("failed to fetch Consul CA: %w", err)
	}

	caPath := filepath.Join(certsDir, "consul-agent-ca.pem")
	if err := os.WriteFile(caPath, caData, 0644); err != nil {
		return fmt.Errorf("failed to write Consul CA: %w", err)
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
