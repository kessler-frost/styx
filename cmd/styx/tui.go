package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kessler-frost/styx/internal/setup"
	"github.com/kessler-frost/styx/internal/tui"
)

// runTUI starts the interactive TUI.
func runTUI() error {
	// Check prerequisites
	prereqs := setup.GetStatus()
	needsSetup := setup.NeedsSetup(prereqs)

	// Create TUI model
	model := tui.New(tui.Options{
		SetupMode: needsSetup,
		Prereqs:   prereqs,
	})

	// Run the TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
