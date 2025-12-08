package api

import (
	"github.com/kessler-frost/styx/internal/network"
	"github.com/kessler-frost/styx/internal/services"
)

// GetPlatformServices returns the status of all platform services.
func (c *Client) GetPlatformServices() ([]PlatformService, error) {
	// Check if Nomad is healthy first
	if c.getNomadStatus().Status != "healthy" {
		return nil, nil
	}

	statuses, err := services.Status()
	if err != nil {
		return nil, err
	}

	// Get Tailscale info for endpoints
	tsInfo := network.GetTailscaleInfo()

	var result []PlatformService
	for _, s := range statuses {
		ps := PlatformService{
			Name:   s.Name,
			Status: s.Status,
			Health: "unknown",
		}

		// Set endpoint based on service type and Tailscale availability
		ps.Endpoint = getServiceEndpoint(s.Name, tsInfo)

		// Determine health from status
		switch s.Status {
		case "running":
			ps.Health = "healthy"
		case "pending":
			ps.Health = "starting"
		case "dead", "not_deployed":
			ps.Health = "stopped"
		case "error":
			ps.Health = "unhealthy"
		}

		result = append(result, ps)
	}

	return result, nil
}

func getServiceEndpoint(name string, tsInfo network.TailscaleInfo) string {
	if tsInfo.Running && tsInfo.DNSName != "" {
		switch name {
		case "traefik":
			return "https://" + tsInfo.DNSName
		case "grafana":
			return "https://" + tsInfo.DNSName + "/grafana"
		case "prometheus":
			return "https://" + tsInfo.DNSName + "/prometheus"
		case "nats":
			return "nats://localhost:4222"
		case "dragonfly":
			return "redis://localhost:6379"
		case "loki":
			return "http://localhost:3100"
		case "promtail":
			return "http://localhost:9080"
		case "postgres":
			return "postgres://localhost:5432"
		case "rustfs":
			return "http://localhost:9000"
		}
	}

	// Fallback to localhost endpoints
	switch name {
	case "traefik":
		return "http://localhost:4200"
	case "grafana":
		return "http://localhost:4200/grafana"
	case "prometheus":
		return "http://localhost:4200/prometheus"
	case "nats":
		return "nats://localhost:4222"
	case "dragonfly":
		return "redis://localhost:6379"
	case "loki":
		return "http://localhost:3100"
	case "promtail":
		return "http://localhost:9080"
	case "postgres":
		return "postgres://localhost:5432"
	case "rustfs":
		return "http://localhost:9000"
	default:
		return ""
	}
}
