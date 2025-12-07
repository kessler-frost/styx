package services

import (
	"fmt"
	"time"

	"github.com/kessler-frost/styx/internal/network"
)

// Service represents a platform service
type Service struct {
	Name        string
	Description string
	JobHCL      string             // Static HCL for simple services
	JobHCLFunc  func(string) string // Dynamic HCL generator (takes Tailscale IP)
}

// ServiceStatus represents the status of a platform service
type ServiceStatus struct {
	Name        string
	Description string
	Status      string // not_deployed, pending, running, dead
}

// PlatformServices is the registry of built-in platform services
var PlatformServices = []Service{
	{
		Name:        "nats",
		Description: "Message queue (NATS)",
		JobHCL:      natsJobHCL,
	},
	{
		Name:        "dragonfly",
		Description: "Redis-compatible cache (Dragonfly)",
		JobHCL:      dragonflyJobHCL,
	},
	{
		Name:        "traefik",
		Description: "Ingress controller (Traefik)",
		JobHCLFunc:  TraefikJobHCL,
	},
}

// Deploy deploys a platform service by name
func Deploy(name string) error {
	client := DefaultClient()

	for _, svc := range PlatformServices {
		if svc.Name == name {
			hcl, err := getServiceHCL(svc)
			if err != nil {
				return fmt.Errorf("failed to generate HCL for %s: %w", name, err)
			}
			return client.RunJob(hcl)
		}
	}

	return fmt.Errorf("unknown service: %s", name)
}

// getServiceHCL returns the HCL for a service, handling dynamic generation if needed
func getServiceHCL(svc Service) (string, error) {
	if svc.JobHCLFunc != nil {
		tsInfo := network.GetTailscaleInfo()
		if !tsInfo.Running {
			return "", fmt.Errorf("tailscale is required for %s but not running", svc.Name)
		}
		return svc.JobHCLFunc(tsInfo.IP), nil
	}
	return svc.JobHCL, nil
}

// Stop stops a platform service by name
func Stop(name string) error {
	client := DefaultClient()

	// Verify it's a known platform service
	found := false
	for _, svc := range PlatformServices {
		if svc.Name == name {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("unknown service: %s", name)
	}

	return client.StopJob(name)
}

// Status returns the status of all platform services
func Status() ([]ServiceStatus, error) {
	client := DefaultClient()

	result := make([]ServiceStatus, len(PlatformServices))

	for i, svc := range PlatformServices {
		result[i] = ServiceStatus{
			Name:        svc.Name,
			Description: svc.Description,
			Status:      "not_deployed",
		}

		jobStatus, err := client.GetJobStatus(svc.Name)
		if err != nil {
			result[i].Status = "error"
			continue
		}

		if jobStatus != nil {
			result[i].Status = jobStatus.Status
		}
	}

	return result, nil
}

// DeployAll deploys all platform services
func DeployAll() error {
	client := DefaultClient()

	for _, svc := range PlatformServices {
		fmt.Printf("  Deploying %s...\n", svc.Name)
		hcl, err := getServiceHCL(svc)
		if err != nil {
			return fmt.Errorf("failed to generate HCL for %s: %w", svc.Name, err)
		}
		if err := client.RunJob(hcl); err != nil {
			return fmt.Errorf("failed to deploy %s: %w", svc.Name, err)
		}
	}

	// Wait for services to become running
	fmt.Println("  Waiting for services to start...")
	return waitForServices(60 * time.Second)
}

// waitForServices waits for all platform services to reach running state
func waitForServices(timeout time.Duration) error {
	client := DefaultClient()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allRunning := true

		for _, svc := range PlatformServices {
			status, err := client.GetJobStatus(svc.Name)
			if err != nil {
				allRunning = false
				break
			}

			if status == nil || status.Status != "running" {
				allRunning = false
				break
			}
		}

		if allRunning {
			return nil
		}

		time.Sleep(2 * time.Second)
		fmt.Print(".")
	}

	fmt.Println()
	return fmt.Errorf("timeout waiting for services to start")
}

// StopAll stops all platform services
func StopAll() error {
	client := DefaultClient()

	for _, svc := range PlatformServices {
		// Check if it's running first
		status, _ := client.GetJobStatus(svc.Name)
		if status == nil {
			continue
		}

		fmt.Printf("  Stopping %s...\n", svc.Name)
		if err := client.StopJob(svc.Name); err != nil {
			return fmt.Errorf("failed to stop %s: %w", svc.Name, err)
		}
	}

	return nil
}

// GetService returns a service by name, or nil if not found
func GetService(name string) *Service {
	for _, svc := range PlatformServices {
		if svc.Name == name {
			return &svc
		}
	}
	return nil
}
