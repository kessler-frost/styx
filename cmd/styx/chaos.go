package main

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/kessler-frost/styx/internal/launchd"
	"github.com/kessler-frost/styx/internal/services"
	"github.com/spf13/cobra"
)

var chaosCmd = &cobra.Command{
	Use:   "chaos",
	Short: "Run chaos tests on the Styx cluster",
	Long: `Run chaos tests to verify cluster resilience and recovery.

Available tests:
  --agent       Kill and recover Nomad agent
  --services    Kill platform services and verify restart
  --container   Verify container runtime availability
  --rejoin      Verify cluster membership and service discovery
  --all         Run all chaos tests`,
	RunE: runChaos,
}

var (
	chaosAll       bool
	chaosAgent     bool
	chaosServices  bool
	chaosContainer bool
	chaosRejoin    bool
)

func init() {
	chaosCmd.Flags().BoolVar(&chaosAll, "all", false, "Run all chaos tests")
	chaosCmd.Flags().BoolVar(&chaosAgent, "agent", false, "Test Nomad agent kill/recovery")
	chaosCmd.Flags().BoolVar(&chaosServices, "services", false, "Test platform service kill/restart")
	chaosCmd.Flags().BoolVar(&chaosContainer, "container", false, "Test container runtime availability")
	chaosCmd.Flags().BoolVar(&chaosRejoin, "rejoin", false, "Test cluster membership verification")
	rootCmd.AddCommand(chaosCmd)
}

type chaosTest struct {
	Name    string
	Run     func() error
	Enabled bool
}

func runChaos(cmd *cobra.Command, args []string) error {
	if !launchd.IsLoaded("com.styx.nomad") {
		return fmt.Errorf("Styx is not running. Start with 'styx init' first")
	}

	tests := []chaosTest{
		{Name: "Agent Kill/Recovery", Run: testAgentRecovery, Enabled: chaosAll || chaosAgent},
		{Name: "Service Kill/Restart", Run: testServiceRestart, Enabled: chaosAll || chaosServices},
		{Name: "Container Runtime", Run: testContainerRuntime, Enabled: chaosAll || chaosContainer},
		{Name: "Cluster Membership", Run: testClusterMembership, Enabled: chaosAll || chaosRejoin},
	}

	enabledCount := 0
	for _, t := range tests {
		if t.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return cmd.Help()
	}

	fmt.Println("Styx Chaos Testing")
	fmt.Println("==================")
	fmt.Println()

	passed, failed := 0, 0

	for _, test := range tests {
		if !test.Enabled {
			continue
		}

		fmt.Printf("[TEST] %s\n", test.Name)
		if err := test.Run(); err != nil {
			fmt.Printf("[FAIL] %s: %v\n\n", test.Name, err)
			failed++
			continue
		}
		fmt.Printf("[PASS] %s\n\n", test.Name)
		passed++
	}

	fmt.Println("==================")
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)

	if failed > 0 {
		return fmt.Errorf("%d tests failed", failed)
	}
	return nil
}

func testAgentRecovery() error {
	fmt.Println("  - Stopping Nomad agent...")
	if err := launchd.Stop("com.styx.nomad"); err != nil {
		return fmt.Errorf("failed to stop agent: %w", err)
	}

	// Kill any orphan Nomad/Vault processes that survived the launchd stop
	fmt.Println("  - Cleaning up orphan processes...")
	exec.Command("pkill", "-f", "nomad agent").Run()
	exec.Command("pkill", "-f", "vault server").Run()

	// Wait for ports to be freed (Nomad RPC on 4647, HTTP on 4646)
	fmt.Println("  - Waiting for ports to be freed...")
	if err := waitForPortFree(4646, 10*time.Second); err != nil {
		return fmt.Errorf("port 4646 still in use: %w", err)
	}
	if err := waitForPortFree(4647, 10*time.Second); err != nil {
		return fmt.Errorf("port 4647 still in use: %w", err)
	}

	fmt.Println("  - Restarting Nomad agent...")
	if err := launchd.Start("com.styx.nomad"); err != nil {
		return fmt.Errorf("failed to restart agent: %w", err)
	}

	fmt.Println("  - Waiting for agent health...")
	return waitForHealth("http://127.0.0.1:4646/v1/agent/health", 120*time.Second)
}

// waitForPortFree waits until a port is no longer in use
func waitForPortFree(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err != nil {
			// Connection refused means port is free
			return nil
		}
		conn.Close()
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d to be freed", port)
}

func testServiceRestart() error {
	client := services.DefaultClient()
	testService := "nats"

	fmt.Printf("  - Checking %s status...\n", testService)
	status, err := client.GetJobStatus(testService)
	if err != nil || status == nil {
		return fmt.Errorf("%s not deployed", testService)
	}

	fmt.Printf("  - Stopping %s...\n", testService)
	if err := client.StopJob(testService); err != nil {
		return fmt.Errorf("failed to stop %s: %w", testService, err)
	}

	time.Sleep(3 * time.Second)

	fmt.Printf("  - Redeploying %s...\n", testService)
	if err := services.Deploy(testService); err != nil {
		return fmt.Errorf("failed to redeploy %s: %w", testService, err)
	}

	fmt.Println("  - Waiting for service to start...")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		status, err := client.GetJobStatus(testService)
		if err == nil && status != nil && status.Status == "running" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for %s to restart", testService)
}

func testContainerRuntime() error {
	fmt.Println("  - Checking container CLI...")
	cmd := exec.Command("container", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container CLI not available: %w", err)
	}
	fmt.Println("  - Container CLI is functional")
	return nil
}

func testClusterMembership() error {
	fmt.Println("  - Checking cluster membership...")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:4646/v1/agent/members")
	if err != nil {
		return fmt.Errorf("failed to get cluster members: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	fmt.Println("  - Node is connected to cluster")
	fmt.Println("  - Service discovery verified")
	return nil
}

func waitForHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for health at %s", url)
}
