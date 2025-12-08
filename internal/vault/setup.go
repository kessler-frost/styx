package vault

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InitOutput holds the output from vault operator init.
type InitOutput struct {
	UnsealKeysB64 []string `json:"unseal_keys_b64"`
	RootToken     string   `json:"root_token"`
}

// Initialize runs vault operator init and saves the unseal keys.
// Returns the root token on success.
func Initialize(secretsDir string) (string, error) {
	// Check if already initialized
	initFile := filepath.Join(secretsDir, "vault-init.json")
	if _, err := os.Stat(initFile); err == nil {
		// Already initialized, load and return root token
		data, err := os.ReadFile(initFile)
		if err != nil {
			return "", fmt.Errorf("failed to read vault init file: %w", err)
		}
		var output InitOutput
		if err := json.Unmarshal(data, &output); err != nil {
			return "", fmt.Errorf("failed to parse vault init file: %w", err)
		}
		return output.RootToken, nil
	}

	// Initialize Vault
	cmd := exec.Command("vault", "operator", "init", "-format=json", "-key-shares=1", "-key-threshold=1")
	cmd.Env = append(os.Environ(), "VAULT_ADDR=http://127.0.0.1:8200")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to initialize vault: %w", err)
	}

	var initOutput InitOutput
	if err := json.Unmarshal(output, &initOutput); err != nil {
		return "", fmt.Errorf("failed to parse vault init output: %w", err)
	}

	// Save init output to secrets directory
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create secrets directory: %w", err)
	}

	if err := os.WriteFile(initFile, output, 0600); err != nil {
		return "", fmt.Errorf("failed to write vault init file: %w", err)
	}

	return initOutput.RootToken, nil
}

// Unseal attempts to unseal Vault using stored unseal keys.
func Unseal(secretsDir string) error {
	initFile := filepath.Join(secretsDir, "vault-init.json")
	data, err := os.ReadFile(initFile)
	if err != nil {
		return fmt.Errorf("failed to read vault init file: %w", err)
	}

	var initOutput InitOutput
	if err := json.Unmarshal(data, &initOutput); err != nil {
		return fmt.Errorf("failed to parse vault init file: %w", err)
	}

	// Unseal with first key
	cmd := exec.Command("vault", "operator", "unseal", initOutput.UnsealKeysB64[0])
	cmd.Env = append(os.Environ(), "VAULT_ADDR=http://127.0.0.1:8200")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unseal vault: %w", err)
	}

	return nil
}

// VaultStatus holds the parsed status from vault status command.
type VaultStatus struct {
	Initialized bool `json:"initialized"`
	Sealed      bool `json:"sealed"`
}

// GetStatus returns the current Vault status.
func GetStatus() (*VaultStatus, error) {
	cmd := exec.Command("vault", "status", "-format=json")
	cmd.Env = append(os.Environ(), "VAULT_ADDR=http://127.0.0.1:8200")

	// vault status returns exit code 2 when sealed, but still outputs JSON
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's just a "sealed" exit code (2) - we can still parse the JSON
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			// Continue to parse the JSON output
		} else {
			return nil, fmt.Errorf("failed to check vault status: %w", err)
		}
	}

	var status VaultStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, fmt.Errorf("failed to parse vault status: %w", err)
	}

	return &status, nil
}

// IsSealed checks if Vault is sealed.
func IsSealed() (bool, error) {
	status, err := GetStatus()
	if err != nil {
		return false, err
	}
	return status.Sealed, nil
}

// IsInitialized checks if Vault is initialized.
func IsInitialized() (bool, error) {
	status, err := GetStatus()
	if err != nil {
		return false, err
	}
	return status.Initialized, nil
}

// GetRootToken returns the root token from stored init output.
func GetRootToken(secretsDir string) (string, error) {
	initFile := filepath.Join(secretsDir, "vault-init.json")
	data, err := os.ReadFile(initFile)
	if err != nil {
		return "", fmt.Errorf("failed to read vault init file: %w", err)
	}

	var initOutput InitOutput
	if err := json.Unmarshal(data, &initOutput); err != nil {
		return "", fmt.Errorf("failed to parse vault init file: %w", err)
	}

	return initOutput.RootToken, nil
}

// generateRandomPassword generates a random password of specified length.
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// waitForNomadJWKS waits for Nomad's JWKS endpoint to become available.
func waitForNomadJWKS(timeout time.Duration) error {
	jwksURL := "http://127.0.0.1:4646/.well-known/jwks.json"
	client := &http.Client{Timeout: 5 * time.Second}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(jwksURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for Nomad JWKS endpoint at %s", jwksURL)
}

// SetupNomadIntegration configures Vault JWT auth for Nomad workload identities.
// This is the modern approach for Nomad 1.7+ that uses short-lived tokens.
func SetupNomadIntegration(secretsDir string) error {
	rootToken, err := GetRootToken(secretsDir)
	if err != nil {
		return err
	}

	env := append(os.Environ(),
		"VAULT_ADDR=http://127.0.0.1:8200",
		"VAULT_TOKEN="+rootToken,
	)

	// Enable KV secrets engine
	cmd := exec.Command("vault", "secrets", "enable", "-path=secret", "kv-v2")
	cmd.Env = env
	// Ignore error if already enabled
	if err := cmd.Run(); err != nil {
		fmt.Printf("Note: KV secrets engine may already be enabled: %v\n", err)
	}

	// Create Nomad workload policy for reading secrets
	nomadPolicy := `
# Allow reading secrets
path "secret/data/*" {
  capabilities = ["read"]
}

path "secret/metadata/*" {
  capabilities = ["read", "list"]
}
`
	policyFile := filepath.Join(secretsDir, "nomad-workloads-policy.hcl")
	if err := os.WriteFile(policyFile, []byte(nomadPolicy), 0600); err != nil {
		return fmt.Errorf("failed to write nomad workloads policy: %w", err)
	}

	cmd = exec.Command("vault", "policy", "write", "nomad-workloads", policyFile)
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create nomad workloads policy: %w", err)
	}

	// Enable JWT auth method for Nomad workload identities
	cmd = exec.Command("vault", "auth", "enable", "-path=jwt-nomad", "jwt")
	cmd.Env = env
	// Ignore error if already enabled
	if err := cmd.Run(); err != nil {
		fmt.Printf("Note: JWT auth method may already be enabled: %v\n", err)
	}

	// Wait for Nomad's JWKS endpoint to be ready before configuring JWT auth
	// Vault validates the JWKS URL when writing config, so it must be available
	if err := waitForNomadJWKS(60 * time.Second); err != nil {
		return fmt.Errorf("Nomad JWKS not available: %w", err)
	}

	// Configure JWT auth with Nomad's JWKS endpoint
	// The JWKS endpoint is served by Nomad at /.well-known/jwks.json
	cmd = exec.Command("vault", "write", "auth/jwt-nomad/config",
		"jwks_url=http://127.0.0.1:4646/.well-known/jwks.json",
		"default_role=nomad-workloads",
	)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure JWT auth: %w\nOutput: %s", err, output)
	}

	// Create a role for Nomad workloads
	// This role maps Nomad workload identities to Vault policies
	cmd = exec.Command("vault", "write", "auth/jwt-nomad/role/nomad-workloads",
		"role_type=jwt",
		"bound_audiences=vault.io",
		"user_claim=/nomad_job_id",
		"user_claim_json_pointer=true",
		"token_type=service",
		"token_policies=nomad-workloads",
		"token_period=30m",
		"token_ttl=1h",
	)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create JWT role: %w\nOutput: %s", err, output)
	}

	// Create random postgres password
	postgresPassword, err := generateRandomPassword(16)
	if err != nil {
		return fmt.Errorf("failed to generate postgres password: %w", err)
	}
	cmd = exec.Command("vault", "kv", "put", "secret/postgres", fmt.Sprintf("password=%s", postgresPassword))
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		fmt.Printf("Note: failed to create default postgres secret: %v\n", err)
	}

	// Create random rustfs credentials
	rustfsAccessKey, err := generateRandomPassword(16)
	if err != nil {
		return fmt.Errorf("failed to generate rustfs access key: %w", err)
	}
	rustfsSecretKey, err := generateRandomPassword(32)
	if err != nil {
		return fmt.Errorf("failed to generate rustfs secret key: %w", err)
	}
	cmd = exec.Command("vault", "kv", "put", "secret/rustfs",
		fmt.Sprintf("access_key=%s", rustfsAccessKey),
		fmt.Sprintf("secret_key=%s", rustfsSecretKey))
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		fmt.Printf("Note: failed to create default rustfs secret: %v\n", err)
	}

	// Create random Grafana admin password
	grafanaPassword, err := generateRandomPassword(16)
	if err != nil {
		return fmt.Errorf("failed to generate grafana password: %w", err)
	}
	cmd = exec.Command("vault", "kv", "put", "secret/grafana", fmt.Sprintf("admin_password=%s", grafanaPassword))
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		fmt.Printf("Note: failed to create default grafana secret: %v\n", err)
	}

	// Save a marker file to indicate workload identity is configured
	markerFile := filepath.Join(secretsDir, "vault-workload-identity-configured")
	if err := os.WriteFile(markerFile, []byte("configured"), 0600); err != nil {
		return fmt.Errorf("failed to write marker file: %w", err)
	}

	return nil
}

// GetNomadToken returns the Vault token for Nomad.
func GetNomadToken(secretsDir string) (string, error) {
	tokenFile := filepath.Join(secretsDir, "nomad-vault-token")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read nomad vault token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
