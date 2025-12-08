package setup

import (
	"fmt"
	"os"
	"os/exec"
)

// InstallResult represents the result of an installation attempt.
type InstallResult struct {
	Name    string
	Success bool
	Output  string
	Error   string
}

// RunCommand executes a shell command and returns the result.
func RunCommand(cmdStr string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// InstallNomad installs Nomad via Homebrew.
func InstallNomad() InstallResult {
	result := InstallResult{Name: "nomad"}

	// First tap hashicorp/tap
	output, err := RunCommand("brew tap hashicorp/tap")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to tap hashicorp/tap: %v\n%s", err, output)
		return result
	}

	// Then install nomad
	output, err = RunCommand("brew install hashicorp/tap/nomad")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to install nomad: %v\n%s", err, output)
		return result
	}

	result.Success = true
	result.Output = output
	return result
}

// InstallVault installs Vault via Homebrew.
func InstallVault() InstallResult {
	result := InstallResult{Name: "vault"}

	// First tap hashicorp/tap (may already be tapped)
	output, err := RunCommand("brew tap hashicorp/tap")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to tap hashicorp/tap: %v\n%s", err, output)
		return result
	}

	// Then install vault
	output, err = RunCommand("brew install hashicorp/tap/vault")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to install vault: %v\n%s", err, output)
		return result
	}

	result.Success = true
	result.Output = output
	return result
}

// InstallContainer installs Apple Container CLI via Homebrew and starts the service.
func InstallContainer() InstallResult {
	result := InstallResult{Name: "container"}

	// Install container
	output, err := RunCommand("brew install container")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to install container: %v\n%s", err, output)
		return result
	}

	// Start the container service
	output, err = RunCommand("brew services start container")
	if err != nil {
		result.Error = fmt.Sprintf("Container installed but failed to start service: %v\n%s", err, output)
		return result
	}

	result.Success = true
	result.Output = output
	return result
}

// StartContainerService starts the container service if it's installed but not running.
func StartContainerService() InstallResult {
	result := InstallResult{Name: "container-service"}

	output, err := RunCommand("brew services start container")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to start container service: %v\n%s", err, output)
		return result
	}

	result.Success = true
	result.Output = output
	return result
}

// InstallTailscale installs Tailscale via Homebrew cask.
func InstallTailscale() InstallResult {
	result := InstallResult{Name: "tailscale"}

	output, err := RunCommand("brew install --cask tailscale")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to install tailscale: %v\n%s", err, output)
		return result
	}

	result.Success = true
	result.Output = "Tailscale installed. Please open the Tailscale app and sign in to your tailnet."
	return result
}

// Install runs the installation for the given prerequisite.
func Install(p Prerequisite) InstallResult {
	switch p.Name {
	case "nomad":
		return InstallNomad()
	case "vault":
		return InstallVault()
	case "container":
		if p.Status == Error {
			// Already installed, just need to start service
			return StartContainerService()
		}
		return InstallContainer()
	case "tailscale":
		return InstallTailscale()
	default:
		return InstallResult{
			Name:  p.Name,
			Error: fmt.Sprintf("Unknown prerequisite: %s", p.Name),
		}
	}
}
