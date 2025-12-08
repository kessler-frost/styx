package api

import (
	"github.com/kessler-frost/styx/internal/launchd"
)

// GetClusterStatus returns the current cluster status.
func (c *Client) GetClusterStatus() ClusterStatus {
	status := ClusterStatus{}

	// Check if service is running
	if !launchd.IsLoaded("com.styx.nomad") {
		status.Service = "stopped"
		return status
	}
	status.Service = "running"

	// Check Vault health
	status.Vault = c.getVaultStatus()

	// Check Nomad health
	status.Nomad = c.getNomadStatus()
	if status.Nomad.Status != "healthy" {
		return status
	}

	// Get agent self info
	var self AgentSelf
	if err := c.get(c.nomadAddr+"/v1/agent/self", &self); err != nil {
		return status
	}

	// Determine mode
	if self.Config.Server.Enabled {
		status.Mode = "server"
	} else {
		status.Mode = "client"
	}

	status.NodeName = self.Config.NodeName
	status.Datacenter = self.Config.Datacenter
	status.Region = self.Config.Region

	// Get cluster members if server
	if status.Mode == "server" {
		status.Members = c.getClusterMembers()
	}

	// Get known servers if client
	if status.Mode == "client" && self.Stats.Client != nil {
		status.KnownServers = self.Stats.Client["known_servers"]
	}

	return status
}

func (c *Client) getVaultStatus() VaultStatus {
	status := VaultStatus{}

	code, err := c.getStatus(c.vaultAddr + "/v1/sys/health")
	if err != nil {
		status.Status = "not_responding"
		return status
	}

	switch code {
	case 200:
		status.Status = "healthy"
		status.Mode = "active"
	case 429:
		status.Status = "healthy"
		status.Mode = "standby"
	case 503:
		status.Status = "sealed"
	default:
		status.Status = "error"
	}

	return status
}

func (c *Client) getNomadStatus() NomadStatus {
	status := NomadStatus{}

	code, err := c.getStatus(c.nomadAddr + "/v1/agent/health")
	if err != nil {
		status.Status = "not_responding"
		return status
	}

	if code == 200 {
		status.Status = "healthy"
	} else {
		status.Status = "unhealthy"
	}

	return status
}

func (c *Client) getClusterMembers() []Member {
	var resp AgentMembers
	if err := c.get(c.nomadAddr+"/v1/agent/members", &resp); err != nil {
		return nil
	}

	var members []Member
	for _, m := range resp.Members {
		members = append(members, Member{
			Name:   m.Name,
			Addr:   m.Addr,
			Port:   m.Port,
			Status: m.Status,
			Role:   m.Tags.Role,
		})
	}

	return members
}
