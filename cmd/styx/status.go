package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Styx cluster status",
	Long:  `Display the current status of the Styx/Nomad service and cluster connectivity.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

type agentSelf struct {
	Config struct {
		Datacenter string `json:"Datacenter"`
		NodeName   string `json:"NodeName"`
		Region     string `json:"Region"`
	} `json:"config"`
	Member struct {
		Name   string `json:"Name"`
		Addr   string `json:"Addr"`
		Port   int    `json:"Port"`
		Status string `json:"Status"`
	} `json:"member"`
	Stats struct {
		Client map[string]string `json:"client"`
		Server map[string]string `json:"server"`
	} `json:"stats"`
}

type agentMembers struct {
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

func runStatus(cmd *cobra.Command, args []string) error {
	label := "com.styx.nomad"

	fmt.Println("Styx Status")
	fmt.Println("-----------")

	// Check if service is loaded
	if !launchd.IsLoaded(label) {
		fmt.Println("Service:     stopped")
		fmt.Println("\nStyx is not running. Use 'styx init' or 'styx join' to start.")
		return nil
	}
	fmt.Println("Service:     running")

	client := &http.Client{Timeout: 2 * time.Second}

	// Check Vault health (only on servers)
	resp, err := client.Get("http://127.0.0.1:8200/v1/sys/health")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("Vault:       healthy (active)")
		} else if resp.StatusCode == 429 {
			fmt.Println("Vault:       healthy (standby)")
		} else if resp.StatusCode == 503 {
			fmt.Println("Vault:       sealed")
		} else {
			fmt.Println("Vault:       not running")
		}
	}

	// Check Nomad health
	resp, err = client.Get("http://127.0.0.1:4646/v1/agent/health")
	if err != nil {
		fmt.Println("Nomad:       not responding")
		fmt.Printf("\nNomad may still be starting. Check logs at: %s\n", logDir)
		return nil
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Nomad:       unhealthy")
		return nil
	}
	fmt.Println("Nomad:       healthy")

	// Get agent self info
	resp, err = client.Get("http://127.0.0.1:4646/v1/agent/self")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var self agentSelf
	if err := json.NewDecoder(resp.Body).Decode(&self); err != nil {
		return nil
	}

	// Determine mode
	isServer := self.Stats.Server != nil && len(self.Stats.Server) > 0
	if isServer {
		fmt.Println("Mode:        server")
	} else {
		fmt.Println("Mode:        client")
	}

	fmt.Printf("Node Name:   %s\n", self.Config.NodeName)
	fmt.Printf("Datacenter:  %s\n", self.Config.Datacenter)
	fmt.Printf("Region:      %s\n", self.Config.Region)

	// Get cluster members if server
	if isServer {
		resp, err = client.Get("http://127.0.0.1:4646/v1/agent/members")
		if err == nil {
			defer resp.Body.Close()
			var members agentMembers
			if err := json.NewDecoder(resp.Body).Decode(&members); err == nil && len(members.Members) > 0 {
				fmt.Println("\nCluster Members:")
				for _, m := range members.Members {
					status := "alive"
					if m.Status != "alive" {
						status = m.Status
					}
					fmt.Printf("  - %s (%s:%d) [%s]\n", m.Name, m.Addr, m.Port, status)
				}
			}
		}
	}

	// Show connected servers if client
	if !isServer && self.Stats.Client != nil {
		if known, ok := self.Stats.Client["known_servers"]; ok && known != "" {
			fmt.Printf("\nConnected Nomad Servers: %s\n", known)
		}
	}

	fmt.Println("\nNomad UI: http://127.0.0.1:4646")
	if isServer {
		fmt.Println("Vault UI: http://127.0.0.1:8200/ui")
	}
	fmt.Println("Transport encryption: Tailscale WireGuard")

	return nil
}
