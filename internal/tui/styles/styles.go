package styles

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#6366F1") // Indigo
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorBorder    = lipgloss.Color("#374151") // Dark gray
)

// Status icons
const (
	IconInstalled = "✓"
	IconMissing   = "✗"
	IconPending   = "○"
	IconError     = "!"
	IconArrow     = "→"
	IconSpinner   = "◐"
)

// Styles for different UI elements
var (
	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginBottom(1)

	// Header bar
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF"))

	// Status indicators
	InstalledStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	MissingStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	PendingStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	// List items
	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Bold(true).
				Foreground(ColorPrimary)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	DialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	// Code/command styles
	CodeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#D1D5DB")).
			Padding(0, 1)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1)

	// Key binding help
	KeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	DescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Divider
	DividerStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	// Tab bar
	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Underline(true)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)
)

// StatusIcon returns the appropriate icon for a status.
func StatusIcon(installed bool, hasError bool) string {
	if hasError {
		return ErrorStyle.Render(IconError)
	}
	if installed {
		return InstalledStyle.Render(IconInstalled)
	}
	return MissingStyle.Render(IconMissing)
}

// RenderKeyHelp renders a key binding with description.
func RenderKeyHelp(key, desc string) string {
	return KeyStyle.Render(key) + " " + DescStyle.Render(desc)
}

// RenderDivider renders a horizontal divider.
func RenderDivider(width int) string {
	divider := ""
	for i := 0; i < width; i++ {
		divider += "─"
	}
	return DividerStyle.Render(divider)
}
