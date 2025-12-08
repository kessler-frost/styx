package views

import (
	"fmt"
	"strings"

	"github.com/kessler-frost/styx/internal/api"
	"github.com/kessler-frost/styx/internal/tui/styles"
)

// JobsModel represents the jobs view.
type JobsModel struct {
	client *api.Client
	jobs   []api.Job
	err    error
}

// NewJobsModel creates a new jobs view model.
func NewJobsModel(client *api.Client) JobsModel {
	m := JobsModel{client: client}
	m.Refresh()
	return m
}

// Refresh fetches the latest jobs.
func (m *JobsModel) Refresh() {
	m.jobs, m.err = m.client.GetJobs()
}

// View renders the jobs view.
func (m JobsModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Jobs"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		return b.String()
	}

	if m.jobs == nil {
		b.WriteString(styles.ErrorStyle.Render("Nomad is not running"))
		b.WriteString("\n\n")
		b.WriteString("Start Styx first with 'styx init'\n")
		return b.String()
	}

	if len(m.jobs) == 0 {
		b.WriteString("No jobs running\n")
		b.WriteString("\n")
		b.WriteString(styles.DescStyle.Render("Run a job with: nomad job run <job.nomad>"))
		return b.String()
	}

	// Group jobs by status
	running := make([]api.Job, 0)
	other := make([]api.Job, 0)

	for _, job := range m.jobs {
		if job.Status == "running" {
			running = append(running, job)
		} else {
			other = append(other, job)
		}
	}

	// Running jobs
	if len(running) > 0 {
		b.WriteString(styles.SubtitleStyle.Render("Running"))
		b.WriteString("\n")
		for _, job := range running {
			b.WriteString(m.renderJob(job))
		}
		b.WriteString("\n")
	}

	// Other jobs
	if len(other) > 0 {
		b.WriteString(styles.SubtitleStyle.Render("Other"))
		b.WriteString("\n")
		for _, job := range other {
			b.WriteString(m.renderJob(job))
		}
	}

	return b.String()
}

func (m JobsModel) renderJob(job api.Job) string {
	var b strings.Builder

	icon := getJobIcon(job.Status)
	jobType := styles.DescStyle.Render("[" + job.Type + "]")
	b.WriteString(fmt.Sprintf("  %s %-20s %s\n", icon, job.Name, jobType))

	// Show allocations
	for _, alloc := range job.Allocations {
		allocIcon := getAllocIcon(alloc.ClientStatus)
		shortID := alloc.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		nodeName := styles.DescStyle.Render("on " + alloc.NodeName)
		b.WriteString(fmt.Sprintf("      %s %s %s\n", allocIcon, shortID, nodeName))
	}

	return b.String()
}

func getJobIcon(status string) string {
	switch status {
	case "running":
		return styles.InstalledStyle.Render(styles.IconInstalled)
	case "pending":
		return styles.PendingStyle.Render(styles.IconPending)
	case "dead":
		return styles.MissingStyle.Render(styles.IconMissing)
	default:
		return styles.DescStyle.Render(styles.IconPending)
	}
}

func getAllocIcon(status string) string {
	switch status {
	case "running":
		return styles.InstalledStyle.Render(styles.IconArrow)
	case "pending":
		return styles.PendingStyle.Render(styles.IconPending)
	case "complete":
		return styles.InstalledStyle.Render(styles.IconInstalled)
	case "failed":
		return styles.ErrorStyle.Render(styles.IconError)
	default:
		return styles.DescStyle.Render("-")
	}
}
