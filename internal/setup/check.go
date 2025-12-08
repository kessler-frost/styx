package setup

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// CheckBrew checks if Homebrew is installed.
func CheckBrew() Prerequisite {
	p := Prerequisite{
		Name:     "homebrew",
		CheckCmd: "which brew",
	}

	_, err := exec.LookPath("brew")
	if err != nil {
		p.Status = Missing
		p.InstallCmds = []string{"Visit https://brew.sh to install Homebrew"}
		return p
	}

	p.Status = Installed
	return p
}

// CheckNomad checks if Nomad is installed.
func CheckNomad() Prerequisite {
	p := Prerequisite{
		Name:     "nomad",
		CheckCmd: "which nomad",
	}

	_, err := exec.LookPath("nomad")
	if err != nil {
		p.Status = Missing
		p.InstallCmds = []string{
			"brew tap hashicorp/tap",
			"brew install hashicorp/tap/nomad",
		}
		return p
	}

	p.Status = Installed
	return p
}

// CheckVault checks if Vault is installed.
func CheckVault() Prerequisite {
	p := Prerequisite{
		Name:     "vault",
		CheckCmd: "which vault",
	}

	_, err := exec.LookPath("vault")
	if err != nil {
		p.Status = Missing
		p.InstallCmds = []string{
			"brew tap hashicorp/tap",
			"brew install hashicorp/tap/vault",
		}
		return p
	}

	p.Status = Installed
	return p
}

// CheckContainer checks if Apple Container CLI is installed and the service is running.
func CheckContainer() Prerequisite {
	p := Prerequisite{
		Name:     "container",
		CheckCmd: "which container",
	}

	_, err := exec.LookPath("container")
	if err != nil {
		p.Status = Missing
		p.InstallCmds = []string{
			"brew install container",
			"brew services start container",
		}
		return p
	}

	// Check if container service is running by trying to list containers
	cmd := exec.Command("container", "list")
	if err := cmd.Run(); err != nil {
		p.Status = Error
		p.Error = "Container service not running"
		p.InstallCmds = []string{"brew services start container"}
		return p
	}

	p.Status = Installed
	return p
}

// tailscaleStatus represents the JSON output from tailscale status --json
type tailscaleStatus struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		DNSName    string   `json:"DNSName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
}

// findTailscaleBin finds the tailscale CLI binary path.
func findTailscaleBin() string {
	// Check common locations
	paths := []string{
		"/usr/local/bin/tailscale",
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
	}

	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
	}

	// Try PATH lookup
	if path, err := exec.LookPath("tailscale"); err == nil {
		return path
	}

	return ""
}

// CheckTailscale checks if Tailscale is installed and connected.
func CheckTailscale() Prerequisite {
	p := Prerequisite{
		Name:     "tailscale",
		CheckCmd: "tailscale status",
	}

	binPath := findTailscaleBin()
	if binPath == "" {
		p.Status = Missing
		p.InstallCmds = []string{
			"brew install --cask tailscale",
			"Open Tailscale app and sign in",
		}
		return p
	}

	// Check connection status
	cmd := exec.Command(binPath, "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		p.Status = Error
		p.Error = "Not logged in. Open Tailscale app and sign in"
		return p
	}

	var status tailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		p.Status = Error
		p.Error = "Failed to parse tailscale status"
		return p
	}

	if status.BackendState != "Running" {
		p.Status = Error
		p.Error = "Tailscale not running. Open Tailscale app"
		return p
	}

	// Add tailnet info to the prerequisite
	p.Status = Installed
	if status.Self.DNSName != "" {
		p.Info = strings.TrimSuffix(status.Self.DNSName, ".")
	}
	if len(status.Self.TailscaleIPs) > 0 {
		p.Info = status.Self.TailscaleIPs[0]
	}

	return p
}
