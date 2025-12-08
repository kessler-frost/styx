package views

import (
	"fmt"
	"strings"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/kessler-frost/styx/internal/tui/styles"
)

// ClusterModel represents the cluster view.
type ClusterModel struct {
	client *api.Client
	status api.ClusterStatus
}

// NewClusterModel creates a new cluster view model.
func NewClusterModel(client *api.Client) ClusterModel {
	m := ClusterModel{client: client}
	m.Refresh()
	return m
}

// Refresh fetches the latest cluster status.
func (m *ClusterModel) Refresh() {
	m.status = m.client.GetClusterStatus()
}

// View renders the cluster view.
func (m ClusterModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Cluster Status"))
	b.WriteString("\n\n")

	// Service status
	if m.status.Service == "stopped" {
		b.WriteString(styles.ErrorStyle.Render("Styx is not running"))
		b.WriteString("\n\n")
		b.WriteString("To start: styx init --serve\n")
		b.WriteString("To join:  styx init --join <ip>\n")
		return b.String()
	}

	// Vault status
	vaultIcon := getStatusIcon(m.status.Vault.Status)
	vaultStatus := m.status.Vault.Status
	if m.status.Vault.Mode != "" {
		vaultStatus = fmt.Sprintf("%s (%s)", m.status.Vault.Status, m.status.Vault.Mode)
	}
	b.WriteString(fmt.Sprintf("  %s Vault:      %s\n", vaultIcon, vaultStatus))

	// Nomad status
	nomadIcon := getStatusIcon(m.status.Nomad.Status)
	b.WriteString(fmt.Sprintf("  %s Nomad:      %s\n", nomadIcon, m.status.Nomad.Status))

	b.WriteString("\n")

	// Node info
	b.WriteString(fmt.Sprintf("  Mode:        %s\n", m.status.Mode))
	b.WriteString(fmt.Sprintf("  Node Name:   %s\n", m.status.NodeName))
	b.WriteString(fmt.Sprintf("  Datacenter:  %s\n", m.status.Datacenter))
	b.WriteString(fmt.Sprintf("  Region:      %s\n", m.status.Region))

	// Cluster members (server mode)
	if m.status.Mode == "server" && len(m.status.Members) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.SubtitleStyle.Render("Cluster Members"))
		b.WriteString("\n")
		for _, member := range m.status.Members {
			icon := getMemberIcon(member.Status)
			b.WriteString(fmt.Sprintf("  %s %s (%s:%d)\n", icon, member.Name, member.Addr, member.Port))
		}
	}

	// Known servers (client mode)
	if m.status.Mode == "client" && m.status.KnownServers != "" {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Connected to: %s\n", m.status.KnownServers))
	}

	return b.String()
}

func getStatusIcon(status string) string {
	switch status {
	case "healthy":
		return styles.InstalledStyle.Render(styles.IconInstalled)
	case "sealed", "unhealthy", "not_responding":
		return styles.ErrorStyle.Render(styles.IconError)
	default:
		return styles.PendingStyle.Render(styles.IconPending)
	}
}

func getMemberIcon(status string) string {
	switch status {
	case "alive":
		return styles.InstalledStyle.Render(styles.IconInstalled)
	case "left", "failed":
		return styles.ErrorStyle.Render(styles.IconMissing)
	default:
		return styles.PendingStyle.Render(styles.IconPending)
	}
}
