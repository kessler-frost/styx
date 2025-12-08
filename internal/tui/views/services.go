package views

import (
	"fmt"
	"strings"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/kessler-frost/styx/internal/tui/styles"
)

// ServicesModel represents the services view.
type ServicesModel struct {
	client   *api.Client
	services []api.PlatformService
	err      error
}

// NewServicesModel creates a new services view model.
func NewServicesModel(client *api.Client) ServicesModel {
	m := ServicesModel{client: client}
	m.Refresh()
	return m
}

// Refresh fetches the latest services status.
func (m *ServicesModel) Refresh() {
	m.services, m.err = m.client.GetPlatformServices()
}

// View renders the services view.
func (m ServicesModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Platform Services"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		return b.String()
	}

	if m.services == nil {
		b.WriteString(styles.ErrorStyle.Render("Nomad is not running"))
		b.WriteString("\n\n")
		b.WriteString("Start Styx first with 'styx init'\n")
		return b.String()
	}

	if len(m.services) == 0 {
		b.WriteString("No platform services defined\n")
		return b.String()
	}

	// Group services by status
	running := make([]api.PlatformService, 0)
	stopped := make([]api.PlatformService, 0)

	for _, svc := range m.services {
		if svc.Status == "running" {
			running = append(running, svc)
		} else {
			stopped = append(stopped, svc)
		}
	}

	// Running services
	if len(running) > 0 {
		b.WriteString(styles.SubtitleStyle.Render("Running"))
		b.WriteString("\n")
		for _, svc := range running {
			icon := styles.InstalledStyle.Render(styles.IconInstalled)
			endpoint := styles.DescStyle.Render(svc.Endpoint)
			b.WriteString(fmt.Sprintf("  %s %-12s %s\n", icon, svc.Name, endpoint))
		}
		b.WriteString("\n")
	}

	// Stopped services
	if len(stopped) > 0 {
		b.WriteString(styles.SubtitleStyle.Render("Stopped"))
		b.WriteString("\n")
		for _, svc := range stopped {
			icon := getServiceIcon(svc.Status)
			status := styles.DescStyle.Render(svc.Status)
			b.WriteString(fmt.Sprintf("  %s %-12s %s\n", icon, svc.Name, status))
		}
	}

	return b.String()
}

func getServiceIcon(status string) string {
	switch status {
	case "running":
		return styles.InstalledStyle.Render(styles.IconInstalled)
	case "pending":
		return styles.PendingStyle.Render(styles.IconPending)
	case "dead", "not_deployed":
		return styles.MissingStyle.Render(styles.IconMissing)
	case "error":
		return styles.ErrorStyle.Render(styles.IconError)
	default:
		return styles.DescStyle.Render(styles.IconPending)
	}
}
