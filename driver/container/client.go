package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Client wraps the Apple container CLI
type Client struct {
	binPath string
}

// NewClient creates a new container client
func NewClient(binPath string) *Client {
	if binPath == "" {
		binPath = "/usr/local/bin/container"
	}
	return &Client{binPath: binPath}
}

// BinPath returns the path to the container binary
func (c *Client) BinPath() string {
	return c.binPath
}

// IsAvailable checks if the container CLI is available
func (c *Client) IsAvailable() bool {
	_, err := exec.LookPath(c.binPath)
	return err == nil
}

// Run starts a new container and returns its ID
func (c *Client) Run(ctx context.Context, opts RunOptions) (string, error) {
	args := []string{"run"}

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	if opts.Detach {
		args = append(args, "-d")
	}

	if opts.Remove {
		args = append(args, "--rm")
	}

	if opts.Memory != "" {
		args = append(args, "-m", opts.Memory)
	}

	if opts.CPUs > 0 {
		args = append(args, "-c", strconv.Itoa(opts.CPUs))
	}

	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}

	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	for _, vol := range opts.Volumes {
		args = append(args, "-v", vol)
	}

	args = append(args, opts.Image)

	if opts.Command != "" {
		args = append(args, opts.Command)
	}

	args = append(args, opts.Args...)

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("container run failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("container run failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Stop stops a running container
func (c *Client) Stop(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, c.binPath, "stop", id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("container stop failed: %s", string(output))
	}
	return nil
}

// Remove removes a container
func (c *Client) Remove(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, c.binPath, "rm", id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("container rm failed: %s", string(output))
	}
	return nil
}

// Kill sends a signal to a container
func (c *Client) Kill(ctx context.Context, id string, signal string) error {
	args := []string{"kill"}
	if signal != "" {
		args = append(args, "-s", signal)
	}
	args = append(args, id)

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("container kill failed: %s", string(output))
	}
	return nil
}

// Inspect returns detailed information about a container
func (c *Client) Inspect(ctx context.Context, id string) (*ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "inspect", id)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("container inspect failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("container inspect failed: %w", err)
	}

	var containers []ContainerInfo
	if err := json.Unmarshal(output, &containers); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	return &containers[0], nil
}

// List returns all containers
func (c *Client) List(ctx context.Context, all bool) ([]ContainerInfo, error) {
	args := []string{"list", "--format", "json"}
	if all {
		args = append(args, "-a")
	}

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("container list failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("container list failed: %w", err)
	}

	var containers []ContainerInfo
	if err := json.Unmarshal(output, &containers); err != nil {
		return nil, fmt.Errorf("failed to parse list output: %w", err)
	}

	return containers, nil
}

// Logs returns the logs of a container
func (c *Client) Logs(ctx context.Context, id string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "logs", id)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start logs command: %w", err)
	}

	return &logReader{cmd: cmd, reader: stdout}, nil
}

// Exec runs a command in a running container
func (c *Client) Exec(ctx context.Context, id string, command []string) ([]byte, error) {
	args := []string{"exec", id}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	return cmd.CombinedOutput()
}

// ExecInteractive runs an interactive command in a container
func (c *Client) ExecInteractive(ctx context.Context, id string, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	args := []string{"exec", "-i", id}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

// Exists checks if a container exists
func (c *Client) Exists(ctx context.Context, id string) bool {
	_, err := c.Inspect(ctx, id)
	return err == nil
}

// IsRunning checks if a container is running
func (c *Client) IsRunning(ctx context.Context, id string) bool {
	info, err := c.Inspect(ctx, id)
	if err != nil {
		return false
	}
	return info.Status == "running"
}

// logReader wraps a command and its stdout for log streaming
type logReader struct {
	cmd    *exec.Cmd
	reader io.Reader
}

func (r *logReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *logReader) Close() error {
	return r.cmd.Wait()
}

// Version returns the container CLI version
func (c *Client) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get version: %s", stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// VolumeExists checks if a named volume exists
func (c *Client) VolumeExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "volume", "ls", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("volume list failed: %w", err)
	}
	// Check if name is in output
	return strings.Contains(string(output), name), nil
}

// VolumeCreate creates a named volume
func (c *Client) VolumeCreate(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, c.binPath, "volume", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("volume create failed: %s", string(output))
	}
	return nil
}

// Stats returns resource usage statistics for a container
func (c *Client) Stats(ctx context.Context, id string) (*ContainerStats, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "stats", id, "--format", "json", "--no-stream")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("container stats failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("container stats failed: %w", err)
	}

	var stats []ContainerStats
	if err := json.Unmarshal(output, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats output: %w", err)
	}

	if len(stats) == 0 {
		return nil, fmt.Errorf("no stats available for container: %s", id)
	}

	return &stats[0], nil
}

// Pull downloads an image from the registry
func (c *Client) Pull(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, c.binPath, "image", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("image pull failed: %s", string(output))
	}
	return nil
}

// DiskUsage returns disk usage statistics for images, containers, and volumes
func (c *Client) DiskUsage(ctx context.Context) (*DiskUsage, error) {
	cmd := exec.CommandContext(ctx, c.binPath, "system", "df", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("system df failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("system df failed: %w", err)
	}

	var usage DiskUsage
	if err := json.Unmarshal(output, &usage); err != nil {
		return nil, fmt.Errorf("failed to parse disk usage: %w", err)
	}

	return &usage, nil
}

// Prune removes unused images to free disk space
func (c *Client) Prune(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.binPath, "image", "prune", "--all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("image prune failed: %s", string(output))
	}
	return nil
}
