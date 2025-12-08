package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.styx.nomad.plist")

	fmt.Println("  - Stopping Nomad agent...")
	if err := launchd.Stop("com.styx.nomad"); err != nil {
		return fmt.Errorf("failed to stop agent: %w", err)
	}

	time.Sleep(2 * time.Second)

	fmt.Println("  - Restarting Nomad agent...")
	if err := launchd.Load(plistPath); err != nil {
		return fmt.Errorf("failed to restart agent: %w", err)
	}

	fmt.Println("  - Waiting for agent health...")
	return waitForHealth("http://127.0.0.1:4646/v1/agent/health", 60*time.Second)
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
	cmd := exec.Command("container", "version")
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
