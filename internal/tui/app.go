package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kessler-frost/styx/internal/api"
	"github.com/kessler-frost/styx/internal/setup"
	tuisetup "github.com/kessler-frost/styx/internal/tui/setup"
	"github.com/kessler-frost/styx/internal/tui/styles"
	"github.com/kessler-frost/styx/internal/tui/views"
)

// View represents the current view in normal mode.
type View int

const (
	ViewCluster View = iota
	ViewServices
	ViewJobs
)

// Mode represents the TUI mode.
type Mode int

const (
	ModeSetup Mode = iota
	ModeNormal
)

// tickMsg is sent on each tick for auto-refresh.
type tickMsg time.Time

// Options configures the TUI.
type Options struct {
	SetupMode bool
	Prereqs   setup.PrereqStatus
}

// Model is the main TUI model.
type Model struct {
	mode       Mode
	view       View
	setupModel tuisetup.Model
	cluster    views.ClusterModel
	services   views.ServicesModel
	jobs       views.JobsModel
	width      int
	height     int
	client     *api.Client
}

// New creates a new TUI model.
func New(opts Options) Model {
	m := Model{
		client: api.NewClient(),
	}

	if opts.SetupMode {
		m.mode = ModeSetup
		m.setupModel = tuisetup.New(opts.Prereqs)
	} else {
		m.mode = ModeNormal
		m.view = ViewCluster
		m.cluster = views.NewClusterModel(m.client)
		m.services = views.NewServicesModel(m.client)
		m.jobs = views.NewJobsModel(m.client)
	}

	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.mode == ModeSetup {
		cmds = append(cmds, m.setupModel.Init())
	} else {
		cmds = append(cmds, m.refreshCmd())
		cmds = append(cmds, m.tickCmd())
	}

	return tea.Batch(cmds...)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.mode == ModeSetup {
			var cmd tea.Cmd
			newSetup, cmd := m.setupModel.Update(msg)
			m.setupModel = newSetup.(tuisetup.Model)
			return m, cmd
		}
		return m, nil

	case tickMsg:
		if m.mode == ModeNormal {
			return m, tea.Batch(m.refreshCmd(), m.tickCmd())
		}
		return m, nil

	default:
		if m.mode == ModeSetup {
			newSetup, cmd := m.setupModel.Update(msg)
			m.setupModel = newSetup.(tuisetup.Model)

			// Check if setup is complete
			if m.setupModel.IsDone() {
				m.mode = ModeNormal
				m.view = ViewCluster
				m.cluster = views.NewClusterModel(m.client)
				m.services = views.NewServicesModel(m.client)
				m.jobs = views.NewJobsModel(m.client)
				return m, tea.Batch(m.refreshCmd(), m.tickCmd())
			}

			return m, cmd
		}
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == ModeSetup {
		newSetup, cmd := m.setupModel.Update(msg)
		m.setupModel = newSetup.(tuisetup.Model)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "1":
		m.view = ViewCluster
		return m, m.refreshCmd()

	case "2":
		m.view = ViewServices
		return m, m.refreshCmd()

	case "3":
		m.view = ViewJobs
		return m, m.refreshCmd()

	case "r":
		return m, m.refreshCmd()

	case "?":
		// Toggle help (could add a help overlay in the future)
		return m, nil
	}

	return m, nil
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		switch m.view {
		case ViewCluster:
			m.cluster.Refresh()
		case ViewServices:
			m.services.Refresh()
		case ViewJobs:
			m.jobs.Refresh()
		}
		return nil
	}
}

// View renders the TUI.
func (m Model) View() string {
	if m.mode == ModeSetup {
		return m.setupModel.View()
	}

	return m.renderNormalMode()
}

func (m Model) renderNormalMode() string {
	var content string

	// Tab bar
	tabs := m.renderTabs()

	// Current view content
	switch m.view {
	case ViewCluster:
		content = m.cluster.View()
	case ViewServices:
		content = m.services.View()
	case ViewJobs:
		content = m.jobs.View()
	}

	// Help
	help := styles.HelpStyle.Render(
		styles.RenderKeyHelp("1", "cluster") + "  " +
			styles.RenderKeyHelp("2", "services") + "  " +
			styles.RenderKeyHelp("3", "jobs") + "  " +
			styles.RenderKeyHelp("r", "refresh") + "  " +
			styles.RenderKeyHelp("q", "quit"),
	)

	return tabs + "\n\n" + content + "\n\n" + help
}

func (m Model) renderTabs() string {
	tabs := []string{"Cluster", "Services", "Jobs"}
	var rendered string

	for i, tab := range tabs {
		if View(i) == m.view {
			rendered += styles.TabActiveStyle.Render(" "+tab+" ")
		} else {
			rendered += styles.TabInactiveStyle.Render(" "+tab+" ")
		}
		if i < len(tabs)-1 {
			rendered += "  "
		}
	}

	return rendered
}
