package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// NomadServer represents a discovered Nomad server.
type NomadServer struct {
	IP       string
	Hostname string
	DNSName  string
}

// DiscoverNomadServers probes Tailscale peers for running Nomad servers.
// Returns a list of discovered servers, or nil if none found.
func DiscoverNomadServers(timeout time.Duration) []NomadServer {
	peers := GetTailscalePeers()
	if len(peers) == 0 {
		return nil
	}

	var servers []NomadServer
	var mu sync.Mutex
	var wg sync.WaitGroup

	client := &http.Client{Timeout: timeout}

	for _, peer := range peers {
		wg.Add(1)
		go func(p TailscalePeer) {
			defer wg.Done()

			if isNomadServer(client, p.IP) {
				mu.Lock()
				servers = append(servers, NomadServer{
					IP:       p.IP,
					Hostname: p.Hostname,
					DNSName:  p.DNSName,
				})
				mu.Unlock()
			}
		}(peer)
	}

	wg.Wait()
	return servers
}

// isNomadServer checks if the given IP is running a Nomad server.
func isNomadServer(client *http.Client, ip string) bool {
	url := fmt.Sprintf("http://%s:4646/v1/agent/members", ip)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Parse response to verify it's a server (has members)
	var result struct {
		Members []struct {
			Name string `json:"Name"`
		} `json:"Members"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	// A server will have at least itself in the members list
	return len(result.Members) > 0
}
