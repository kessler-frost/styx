package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kessler-frost/styx/internal/setup"
	"github.com/kessler-frost/styx/internal/tui/styles"
)

// installResultMsg is sent when an installation completes.
type installResultMsg struct {
	result setup.InstallResult
}

// recheckMsg is sent to trigger a prerequisite recheck.
type recheckMsg struct{}

// Model is the bubbletea model for the setup view.
type Model struct {
	prereqs    setup.PrereqStatus
	cursor     int
	installing bool
	installAll bool
	spinner    spinner.Model
	width      int
	height     int
	done       bool
	error      string
}

// New creates a new setup model.
func New(prereqs setup.PrereqStatus) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	return Model{
		prereqs: prereqs,
		spinner: s,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case installResultMsg:
		m.installing = false
		if !msg.result.Success {
			m.error = msg.result.Error
			return m, nil
		}
		// Recheck prerequisites
		return m, m.recheckCmd()

	case recheckMsg:
		m.prereqs = setup.GetStatus()
		m.error = ""
		if !setup.NeedsSetup(m.prereqs) {
			m.done = true
		} else if m.installAll {
			// Continue installing next missing prerequisite
			return m, m.installNextCmd()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.installing {
		return m, nil // Ignore keys while installing
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "y":
		// Install current item
		missing := m.prereqs.MissingPrereqs()
		if len(missing) > 0 && m.cursor < len(missing) {
			m.installing = true
			m.installAll = false
			return m, m.installCmd(missing[m.cursor])
		}

	case "a":
		// Install all
		missing := m.prereqs.MissingPrereqs()
		if len(missing) > 0 {
			m.installing = true
			m.installAll = true
			return m, m.installCmd(missing[0])
		}

	case "n":
		// Skip current item
		missing := m.prereqs.MissingPrereqs()
		if m.cursor < len(missing)-1 {
			m.cursor++
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		missing := m.prereqs.MissingPrereqs()
		if m.cursor < len(missing)-1 {
			m.cursor++
		}

	case "enter":
		if m.done {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) installCmd(p setup.Prerequisite) tea.Cmd {
	return func() tea.Msg {
		result := setup.Install(p)
		return installResultMsg{result: result}
	}
}

func (m Model) installNextCmd() tea.Cmd {
	missing := m.prereqs.MissingPrereqs()
	if len(missing) == 0 {
		return m.recheckCmd()
	}
	return m.installCmd(missing[0])
}

func (m Model) recheckCmd() tea.Cmd {
	return func() tea.Msg {
		return recheckMsg{}
	}
}

// View renders the view.
func (m Model) View() string {
	if m.done {
		return m.renderComplete()
	}

	var b strings.Builder

	// Header
	b.WriteString(styles.HeaderStyle.Render("  Styx Setup  "))
	b.WriteString("\n\n")

	// Status message
	if m.installing {
		b.WriteString(m.spinner.View() + " Installing...\n\n")
	} else {
		b.WriteString("Checking prerequisites...\n\n")
	}

	// Prerequisite list
	b.WriteString(m.renderPrereqList())
	b.WriteString("\n")

	// Error message
	if m.error != "" {
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.error))
		b.WriteString("\n")
	}

	// Installation prompt for current item
	if !m.installing {
		missing := m.prereqs.MissingPrereqs()
		if len(missing) > 0 && m.cursor < len(missing) {
			b.WriteString("\n")
			b.WriteString(m.renderInstallPrompt(missing[m.cursor]))
		}
	}

	// Help
	b.WriteString("\n\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderPrereqList() string {
	var b strings.Builder

	prereqs := m.prereqs.AllPrereqs()
	missing := m.prereqs.MissingPrereqs()

	for _, p := range prereqs {
		// Determine icon and style
		var icon, status string
		switch p.Status {
		case setup.Installed:
			icon = styles.InstalledStyle.Render(styles.IconInstalled)
			status = styles.InstalledStyle.Render("Installed")
			if p.Info != "" {
				status += " " + styles.DescStyle.Render("("+p.Info+")")
			}
		case setup.Missing:
			icon = styles.MissingStyle.Render(styles.IconMissing)
			status = styles.MissingStyle.Render("Not installed")
		case setup.Pending:
			icon = styles.PendingStyle.Render(styles.IconPending)
			status = styles.PendingStyle.Render(p.Error)
		case setup.Error:
			icon = styles.ErrorStyle.Render(styles.IconError)
			status = styles.ErrorStyle.Render(p.Error)
		}

		// Check if this is the selected missing item
		isSelected := false
		for i, mp := range missing {
			if mp.Name == p.Name && i == m.cursor {
				isSelected = true
				break
			}
		}

		// Render line
		name := fmt.Sprintf("%-12s", p.Name)
		if isSelected && !m.installing {
			name = styles.SelectedItemStyle.Render(name)
		}

		b.WriteString(fmt.Sprintf("  %s %s %s\n", icon, name, status))
	}

	return b.String()
}

func (m Model) renderInstallPrompt(p setup.Prerequisite) string {
	var b strings.Builder

	b.WriteString(styles.RenderDivider(50))
	b.WriteString("\n\n")

	// Prompt
	b.WriteString(fmt.Sprintf("Install %s?\n\n", styles.SelectedItemStyle.Render(p.Name)))

	// Commands to run
	if len(p.InstallCmds) > 0 {
		b.WriteString("Will run:\n")
		for _, cmd := range p.InstallCmds {
			b.WriteString("  " + styles.CodeStyle.Render(cmd) + "\n")
		}
	}

	return b.String()
}

func (m Model) renderHelp() string {
	if m.installing {
		return styles.HelpStyle.Render("Installing... please wait")
	}

	return styles.HelpStyle.Render(
		styles.RenderKeyHelp("y", "install") + "  " +
			styles.RenderKeyHelp("a", "install all") + "  " +
			styles.RenderKeyHelp("n", "skip") + "  " +
			styles.RenderKeyHelp("q", "quit"),
	)
}

func (m Model) renderComplete() string {
	var b strings.Builder

	b.WriteString(styles.HeaderStyle.Render("  Styx Setup  "))
	b.WriteString("\n\n")

	b.WriteString(styles.InstalledStyle.Render("All prerequisites installed!"))
	b.WriteString("\n\n")

	b.WriteString(m.renderPrereqList())

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render(styles.RenderKeyHelp("enter", "continue") + "  " + styles.RenderKeyHelp("q", "quit")))

	return b.String()
}

// IsDone returns true if setup is complete.
func (m Model) IsDone() bool {
	return m.done
}
