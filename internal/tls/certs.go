package tls

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CertPaths holds the paths to TLS certificate files.
type CertPaths struct {
	CAFile   string // Path to CA certificate (consul-agent-ca.pem)
	CertFile string // Path to node certificate
	KeyFile  string // Path to node private key
}

// GenerateCA generates a new Certificate Authority using Consul's built-in CA.
// The CA files are created in the specified directory.
func GenerateCA(certsDir string) error {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Check if CA already exists
	caFile := filepath.Join(certsDir, "consul-agent-ca.pem")
	if _, err := os.Stat(caFile); err == nil {
		return nil // CA already exists
	}

	// Generate CA using consul tls ca create
	cmd := exec.Command("consul", "tls", "ca", "create")
	cmd.Dir = certsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w\nOutput: %s", err, output)
	}

	return nil
}

// GenerateServerCert generates a server certificate for Consul.
// Existing certs are deleted and regenerated.
func GenerateServerCert(certsDir, datacenter string) (*CertPaths, error) {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Delete existing server certs (allows reinit without manual cleanup)
	deleteExistingCerts(certsDir, datacenter+"-server-consul")

	// Generate server certificate
	cmd := exec.Command("consul", "tls", "cert", "create", "-server", "-dc", datacenter)
	cmd.Dir = certsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server cert: %w\nOutput: %s", err, output)
	}

	// Find the generated cert files
	certFile, keyFile, err := findLatestCert(certsDir, datacenter+"-server-consul")
	if err != nil {
		return nil, err
	}

	return &CertPaths{
		CAFile:   filepath.Join(certsDir, "consul-agent-ca.pem"),
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

// GenerateClientCert generates a client certificate for Consul.
// Existing certs are deleted and regenerated.
func GenerateClientCert(certsDir, datacenter string) (*CertPaths, error) {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Delete existing client certs (allows reinit without manual cleanup)
	deleteExistingCerts(certsDir, datacenter+"-client-consul")

	// Generate client certificate
	cmd := exec.Command("consul", "tls", "cert", "create", "-client", "-dc", datacenter)
	cmd.Dir = certsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to generate client cert: %w\nOutput: %s", err, output)
	}

	// Find the generated cert files
	certFile, keyFile, err := findLatestCert(certsDir, datacenter+"-client-consul")
	if err != nil {
		return nil, err
	}

	return &CertPaths{
		CAFile:   filepath.Join(certsDir, "consul-agent-ca.pem"),
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

// GenerateGossipKey generates a gossip encryption key using consul keygen.
func GenerateGossipKey() (string, error) {
	cmd := exec.Command("consul", "keygen")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate gossip key: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SaveGossipKey saves the gossip key to a file.
func SaveGossipKey(secretsDir, key string) error {
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	keyFile := filepath.Join(secretsDir, "gossip.key")
	if err := os.WriteFile(keyFile, []byte(key), 0600); err != nil {
		return fmt.Errorf("failed to write gossip key: %w", err)
	}

	return nil
}

// LoadGossipKey loads the gossip key from a file.
func LoadGossipKey(secretsDir string) (string, error) {
	keyFile := filepath.Join(secretsDir, "gossip.key")
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return "", fmt.Errorf("failed to read gossip key: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// CopyCAFromServer copies the CA certificate from the server.
// This is used by client nodes joining the cluster.
func CopyCAFromServer(serverAddr, certsDir string) error {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Download CA from server's HTTP endpoint
	// Note: This requires the server to expose the CA file via HTTP
	// For now, we'll use a simpler approach - fetch via HTTP API
	caURL := fmt.Sprintf("http://%s:8500/v1/connect/ca/roots", serverAddr)

	cmd := exec.Command("curl", "-s", caURL)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to fetch CA from server: %w", err)
	}

	// The API returns JSON with PEM certificates
	// For simplicity, we'll extract just the root cert
	// TODO: Parse JSON properly if needed
	_ = output

	// Alternative: Use SCP or manual copy for now
	return fmt.Errorf("CA distribution not yet implemented - please copy consul-agent-ca.pem from server to %s", certsDir)
}

// findLatestCert finds the most recently created certificate files matching the prefix.
func findLatestCert(dir, prefix string) (certFile, keyFile string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read certs directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, "-key.pem") {
			certFile = filepath.Join(dir, name)
			keyFile = filepath.Join(dir, strings.TrimSuffix(name, ".pem")+"-key.pem")
			break
		}
	}

	if certFile == "" {
		return "", "", fmt.Errorf("no certificate found with prefix %s in %s", prefix, dir)
	}

	return certFile, keyFile, nil
}

// deleteExistingCerts removes existing certificate files matching the prefix.
// This allows regenerating certs without manual cleanup.
func deleteExistingCerts(dir, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // Directory doesn't exist or can't be read, nothing to delete
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".pem") {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// GetExistingCerts returns paths to existing certificates if they exist.
func GetExistingCerts(certsDir, datacenter string, isServer bool) (*CertPaths, error) {
	caFile := filepath.Join(certsDir, "consul-agent-ca.pem")
	if _, err := os.Stat(caFile); err != nil {
		return nil, fmt.Errorf("CA file not found: %w", err)
	}

	var prefix string
	if isServer {
		prefix = datacenter + "-server-consul"
	} else {
		prefix = datacenter + "-client-consul"
	}

	certFile, keyFile, err := findLatestCert(certsDir, prefix)
	if err != nil {
		return nil, err
	}

	return &CertPaths{
		CAFile:   caFile,
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

// NomadCertPaths holds the paths to Nomad TLS certificate files.
type NomadCertPaths struct {
	CAFile   string // Path to Nomad CA certificate (nomad-agent-ca.pem)
	CertFile string // Path to node certificate
	KeyFile  string // Path to node private key
}

// GenerateNomadCA generates a new Certificate Authority for Nomad.
func GenerateNomadCA(certsDir string) error {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Check if Nomad CA already exists
	caFile := filepath.Join(certsDir, "nomad-agent-ca.pem")
	if _, err := os.Stat(caFile); err == nil {
		return nil // CA already exists
	}

	// Generate CA using nomad tls ca create
	cmd := exec.Command("nomad", "tls", "ca", "create")
	cmd.Dir = certsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate Nomad CA: %w\nOutput: %s", err, output)
	}

	return nil
}

// GenerateNomadServerCert generates a server certificate for Nomad.
// Existing certs are deleted and regenerated.
func GenerateNomadServerCert(certsDir, region string) (*NomadCertPaths, error) {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Delete existing server certs (allows reinit without manual cleanup)
	deleteExistingCerts(certsDir, region+"-server-nomad")

	// Generate server certificate
	cmd := exec.Command("nomad", "tls", "cert", "create", "-server", "-region", region)
	cmd.Dir = certsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to generate Nomad server cert: %w\nOutput: %s", err, output)
	}

	// Find the generated cert files
	certFile, keyFile, err := findLatestCert(certsDir, region+"-server-nomad")
	if err != nil {
		return nil, err
	}

	return &NomadCertPaths{
		CAFile:   filepath.Join(certsDir, "nomad-agent-ca.pem"),
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

// GenerateNomadClientCert generates a client certificate for Nomad.
// Existing certs are deleted and regenerated.
func GenerateNomadClientCert(certsDir, region string) (*NomadCertPaths, error) {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Delete existing client certs (allows reinit without manual cleanup)
	deleteExistingCerts(certsDir, region+"-client-nomad")

	// Generate client certificate
	cmd := exec.Command("nomad", "tls", "cert", "create", "-client", "-region", region)
	cmd.Dir = certsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to generate Nomad client cert: %w\nOutput: %s", err, output)
	}

	// Find the generated cert files
	certFile, keyFile, err := findLatestCert(certsDir, region+"-client-nomad")
	if err != nil {
		return nil, err
	}

	return &NomadCertPaths{
		CAFile:   filepath.Join(certsDir, "nomad-agent-ca.pem"),
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

// GetExistingNomadCerts returns paths to existing Nomad certificates if they exist.
func GetExistingNomadCerts(certsDir, region string, isServer bool) (*NomadCertPaths, error) {
	caFile := filepath.Join(certsDir, "nomad-agent-ca.pem")
	if _, err := os.Stat(caFile); err != nil {
		return nil, fmt.Errorf("Nomad CA file not found: %w", err)
	}

	var prefix string
	if isServer {
		prefix = region + "-server-nomad"
	} else {
		prefix = region + "-client-nomad"
	}

	certFile, keyFile, err := findLatestCert(certsDir, prefix)
	if err != nil {
		return nil, err
	}

	return &NomadCertPaths{
		CAFile:   caFile,
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}
