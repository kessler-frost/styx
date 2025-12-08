package setup

// Status represents the installation state of a prerequisite.
type Status int

const (
	Missing   Status = iota // Not installed
	Pending                 // Waiting for dependency
	Installed               // Ready to use
	Error                   // Installed but not working
)

func (s Status) String() string {
	switch s {
	case Missing:
		return "missing"
	case Pending:
		return "pending"
	case Installed:
		return "installed"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// Prerequisite represents a tool/service that Styx depends on.
type Prerequisite struct {
	Name        string   `json:"name"`
	Status      Status   `json:"status"`
	CheckCmd    string   `json:"check_cmd,omitempty"`
	InstallCmds []string `json:"install_cmds,omitempty"`
	Error       string   `json:"error,omitempty"`
	Info        string   `json:"info,omitempty"` // Additional info (e.g., tailscale IP)
}

// PrereqStatus contains the status of all prerequisites.
type PrereqStatus struct {
	Homebrew  Prerequisite `json:"homebrew"`
	Nomad     Prerequisite `json:"nomad"`
	Vault     Prerequisite `json:"vault"`
	Container Prerequisite `json:"container"`
	Tailscale Prerequisite `json:"tailscale"`
}

// GetStatus checks all prerequisites and returns their current status.
func GetStatus() PrereqStatus {
	status := PrereqStatus{}

	// Check Homebrew first (other installs depend on it)
	status.Homebrew = CheckBrew()

	// If Homebrew is missing, mark others as pending
	if status.Homebrew.Status != Installed {
		status.Nomad = Prerequisite{
			Name:   "nomad",
			Status: Pending,
			Error:  "Requires Homebrew",
		}
		status.Vault = Prerequisite{
			Name:   "vault",
			Status: Pending,
			Error:  "Requires Homebrew",
		}
		status.Container = Prerequisite{
			Name:   "container",
			Status: Pending,
			Error:  "Requires Homebrew",
		}
		status.Tailscale = Prerequisite{
			Name:   "tailscale",
			Status: Pending,
			Error:  "Requires Homebrew",
		}
		return status
	}

	// Check all other prerequisites
	status.Nomad = CheckNomad()
	status.Vault = CheckVault()
	status.Container = CheckContainer()
	status.Tailscale = CheckTailscale()

	return status
}

// NeedsSetup returns true if any prerequisite is not installed.
func NeedsSetup(s PrereqStatus) bool {
	return s.Homebrew.Status != Installed ||
		s.Nomad.Status != Installed ||
		s.Vault.Status != Installed ||
		s.Container.Status != Installed ||
		s.Tailscale.Status != Installed
}

// AllPrereqs returns all prerequisites as a slice for iteration.
func (s PrereqStatus) AllPrereqs() []Prerequisite {
	return []Prerequisite{
		s.Homebrew,
		s.Nomad,
		s.Vault,
		s.Container,
		s.Tailscale,
	}
}

// MissingPrereqs returns only the prerequisites that need installation.
func (s PrereqStatus) MissingPrereqs() []Prerequisite {
	var missing []Prerequisite
	for _, p := range s.AllPrereqs() {
		if p.Status != Installed {
			missing = append(missing, p)
		}
	}
	return missing
}
