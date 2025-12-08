package api

import "time"

// ClusterStatus represents the overall cluster status.
type ClusterStatus struct {
	Service     string       `json:"service"`      // running, stopped
	Vault       VaultStatus  `json:"vault"`        // Vault health status
	Nomad       NomadStatus  `json:"nomad"`        // Nomad health status
	Mode        string       `json:"mode"`         // server or client
	NodeName    string       `json:"node_name"`    // Local node name
	Datacenter  string       `json:"datacenter"`   // Datacenter name
	Region      string       `json:"region"`       // Region name
	Members     []Member     `json:"members"`      // Cluster members (servers only)
	KnownServers string      `json:"known_servers"` // Connected servers (clients only)
}

// VaultStatus represents Vault health.
type VaultStatus struct {
	Status  string `json:"status"`  // healthy, sealed, not_responding
	Mode    string `json:"mode"`    // active, standby, ""
}

// NomadStatus represents Nomad health.
type NomadStatus struct {
	Status string `json:"status"` // healthy, unhealthy, not_responding
}

// Member represents a cluster member.
type Member struct {
	Name   string `json:"name"`
	Addr   string `json:"addr"`
	Port   int    `json:"port"`
	Status string `json:"status"` // alive, left, failed
	Role   string `json:"role"`   // server, client
}

// PlatformService represents a platform service (Traefik, Grafana, etc).
type PlatformService struct {
	Name     string `json:"name"`
	Status   string `json:"status"`    // running, stopped, pending
	Endpoint string `json:"endpoint"`  // URL to access the service
	Health   string `json:"health"`    // healthy, unhealthy, unknown
}

// Job represents a Nomad job.
type Job struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`        // service, batch, system
	Status      string      `json:"status"`      // running, pending, dead
	Allocations []Alloc     `json:"allocations"`
	SubmitTime  time.Time   `json:"submit_time"`
}

// Alloc represents a Nomad allocation.
type Alloc struct {
	ID           string `json:"id"`
	NodeID       string `json:"node_id"`
	NodeName     string `json:"node_name"`
	TaskGroup    string `json:"task_group"`
	ClientStatus string `json:"client_status"` // running, pending, complete, failed
	DesiredStatus string `json:"desired_status"`
}

// Node represents a Nomad client node.
type Node struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Status     string `json:"status"`      // ready, down
	Datacenter string `json:"datacenter"`
	NodeClass  string `json:"node_class"`
	Drain      bool   `json:"drain"`
}

// AgentSelf is the response from /v1/agent/self.
type AgentSelf struct {
	Config struct {
		Datacenter string `json:"Datacenter"`
		NodeName   string `json:"NodeName"`
		Region     string `json:"Region"`
		Server     struct {
			Enabled bool `json:"Enabled"`
		} `json:"Server"`
		Client struct {
			Enabled bool `json:"Enabled"`
		} `json:"Client"`
	} `json:"config"`
	Member struct {
		Name   string `json:"Name"`
		Addr   string `json:"Addr"`
		Port   int    `json:"Port"`
		Status string `json:"Status"`
	} `json:"member"`
	Stats struct {
		Client map[string]string `json:"client"`
	} `json:"stats"`
}

// AgentMembers is the response from /v1/agent/members.
type AgentMembers struct {
	Members []struct {
		Name   string `json:"Name"`
		Addr   string `json:"Addr"`
		Port   int    `json:"Port"`
		Status string `json:"Status"`
		Tags   struct {
			Role string `json:"role"`
		} `json:"Tags"`
	} `json:"Members"`
}

// JobListStub is the response from /v1/jobs.
type JobListStub struct {
	ID          string `json:"ID"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	Status      string `json:"Status"`
	SubmitTime  int64  `json:"SubmitTime"` // nanoseconds
}

// AllocListStub is the response from /v1/job/:id/allocations.
type AllocListStub struct {
	ID            string `json:"ID"`
	NodeID        string `json:"NodeID"`
	NodeName      string `json:"NodeName"`
	TaskGroup     string `json:"TaskGroup"`
	ClientStatus  string `json:"ClientStatus"`
	DesiredStatus string `json:"DesiredStatus"`
}

// NodeListStub is the response from /v1/nodes.
type NodeListStub struct {
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	Address    string `json:"Address"`
	Status     string `json:"Status"`
	Datacenter string `json:"Datacenter"`
	NodeClass  string `json:"NodeClass"`
	Drain      bool   `json:"Drain"`
}
