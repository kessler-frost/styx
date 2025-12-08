// Package main implements a Nomad task driver for Apple Containers.
// It manages the lifecycle of containers using Apple's container CLI tool,
// providing orchestration capabilities for macOS containerized workloads.
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/kessler-frost/styx/driver/container"
	"github.com/kessler-frost/styx/internal/network"
)

const (
	pluginName        = "apple-container"
	pluginVersion     = "v0.1.0"
	fingerprintPeriod = 30 * time.Second
	taskHandleVersion = 1
)

var (
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     pluginVersion,
		Name:              pluginName,
	}

	driverCapabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
	}
)

// Driver implements the Nomad task driver plugin interface for Apple Containers.
// It handles starting, stopping, and monitoring containers on macOS hosts
// using the native container CLI provided by Apple.
type Driver struct {
	eventer *eventer.Eventer
	config  *Config
	client  *container.Client
	logger  hclog.Logger

	// tasks is a map of task IDs to task handles
	tasks     map[string]*taskHandle
	tasksLock sync.RWMutex

	// ctx and cancel for the driver
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDriver creates a new Apple Container driver instance.
// The logger should be configured for the Nomad plugin system.
func NewDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &Driver{
		eventer: eventer.NewEventer(ctx, logger),
		logger:  logger.Named(pluginName),
		tasks:   make(map[string]*taskHandle),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// PluginInfo returns metadata about the driver plugin including
// its name, version, and type for the Nomad plugin system.
func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the HCL specification for the driver's plugin-level configuration.
// This defines the schema for configuration in the Nomad client's plugin block.
func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig is called by Nomad to set the driver's plugin-level configuration.
// It decodes the configuration and initializes the container client.
func (d *Driver) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return fmt.Errorf("failed to decode driver config: %w", err)
		}
	}

	d.config = &config
	d.client = container.NewClient(config.ContainerBinPath)

	return nil
}

// TaskConfigSchema returns the HCL specification for task-level configuration.
// This defines the schema for the driver config block in Nomad job specifications.
func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

// Capabilities returns the feature set supported by this driver,
// including signal handling and command execution capabilities.
func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return driverCapabilities, nil
}

// Fingerprint streams periodic health status updates about the driver.
// It checks for the availability of the container CLI and reports version information.
func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil
}

func (d *Driver) handleFingerprint(ctx context.Context, ch chan<- *drivers.Fingerprint) {
	defer close(ch)

	ticker := time.NewTicker(fingerprintPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			ch <- d.buildFingerprint()
		}
	}
}

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	fp := &drivers.Fingerprint{
		Attributes:        map[string]*structs.Attribute{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: "healthy",
	}

	if d.client == nil || !d.client.IsAvailable() {
		fp.Health = drivers.HealthStateUndetected
		fp.HealthDescription = "container CLI not found"
		return fp
	}

	version, err := d.client.Version(d.ctx)
	if err != nil {
		fp.Health = drivers.HealthStateUnhealthy
		fp.HealthDescription = fmt.Sprintf("failed to get container version: %v", err)
		return fp
	}

	fp.Attributes["driver.apple-container.version"] = structs.NewStringAttribute(version)
	fp.Attributes["driver.apple-container"] = structs.NewBoolAttribute(true)

	return fp
}

// StartTask launches a new container based on the provided task configuration.
// It returns a TaskHandle for lifecycle management and a DriverNetwork containing
// the container's network coordinates (IP, hostname) for service registration.
// The driver automatically mounts Nomad's template, secrets, and alloc directories.
func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if cfg.Resources == nil {
		return nil, nil, fmt.Errorf("task resources are required")
	}

	var taskConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&taskConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %w", err)
	}

	d.logger.Info("starting task", "task_id", cfg.ID, "image", taskConfig.Image)

	// Get task directories for auto-mounting (like Docker driver)
	// Templates render to LocalDir, secrets to SecretsDir
	taskDir := cfg.TaskDir()
	autoMounts := []string{
		fmt.Sprintf("%s:/local", taskDir.LocalDir),
		fmt.Sprintf("%s:/secrets", taskDir.SecretsDir),
		fmt.Sprintf("%s:/alloc", taskDir.SharedAllocDir),
	}

	// Auto-create named volumes (volumes that don't start with /)
	for _, vol := range taskConfig.Volumes {
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) < 2 {
			continue
		}
		source := parts[0]
		// If source doesn't start with /, it's a named volume
		if !strings.HasPrefix(source, "/") {
			exists, err := d.client.VolumeExists(d.ctx, source)
			if err != nil {
				d.logger.Warn("failed to check volume existence", "volume", source, "error", err)
			}
			if !exists {
				d.logger.Info("creating named volume", "volume", source)
				if err := d.client.VolumeCreate(d.ctx, source); err != nil {
					return nil, nil, fmt.Errorf("failed to create volume %s: %w", source, err)
				}
			}
		}
	}

	// Merge: auto-mounts first, then user volumes (user can override)
	allVolumes := append(autoMounts, taskConfig.Volumes...)
	d.logger.Debug("volume mounts", "auto", autoMounts, "user", taskConfig.Volumes, "all", allVolumes)

	// Pre-pull image with retry logic
	d.logger.Info("pulling image", "image", taskConfig.Image)
	for attempt := 0; attempt < 3; attempt++ {
		if err := d.client.Pull(d.ctx, taskConfig.Image); err != nil {
			if attempt == 2 {
				return nil, nil, fmt.Errorf("failed to pull image after 3 attempts: %w", err)
			}
			d.logger.Warn("image pull failed, retrying", "attempt", attempt+1, "error", err)
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
			continue
		}
		break
	}

	// Sanitize container name - Apple container CLI doesn't allow slashes
	containerName := strings.ReplaceAll(cfg.ID, "/", "-")

	// Build run options
	// Always use styx network for container-to-container communication
	containerNetwork := taskConfig.Network
	if containerNetwork == "" {
		containerNetwork = network.StyxNetworkName
	}

	opts := container.RunOptions{
		Name:       containerName,
		Image:      taskConfig.Image,
		Command:    taskConfig.Command,
		Args:       taskConfig.Args,
		Env:        taskConfig.Env,
		Ports:      taskConfig.Ports,
		Volumes:    allVolumes,
		Memory:     taskConfig.Memory,
		CPUs:       taskConfig.CPUs,
		WorkingDir: taskConfig.WorkingDir,
		Network:    containerNetwork,
		Detach:     true,
	}

	// Start the container
	containerID, err := d.client.Run(d.ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start container: %w", err)
	}

	d.logger.Info("container started", "container_id", containerID)

	// Track if we encounter an error after container starts - cleanup on failure
	var startErr error
	defer func() {
		if startErr != nil {
			d.logger.Warn("cleaning up container after start failure", "container_id", containerID, "error", startErr)
			if err := d.client.Stop(d.ctx, containerID); err != nil {
				d.logger.Warn("failed to stop container during cleanup", "container_id", containerID, "error", err)
			}
			if err := d.client.Remove(d.ctx, containerID); err != nil {
				d.logger.Warn("failed to remove container during cleanup", "container_id", containerID, "error", err)
			}
		}
	}()

	// Get container network info for service registration
	var driverNetwork *drivers.DriverNetwork
	var containerIP string
	info, err := d.client.Inspect(d.ctx, containerID)
	if err != nil {
		d.logger.Warn("failed to inspect container for network info", "error", err)
	} else if len(info.Networks) > 0 {
		// Extract IP from address (format: "192.168.64.4/24")
		addr := info.Networks[0].Address
		if idx := strings.Index(addr, "/"); idx > 0 {
			addr = addr[:idx]
		}
		containerIP = addr
		d.logger.Info("container network info", "ip", containerIP)
	}

	// Create handle for task lifecycle management
	handle := newTaskHandle(d.client, d.logger, containerID, &taskConfig)

	// Get Tailscale info for cross-node networking
	tailscale := network.GetTailscaleInfo()
	if tailscale.Running {
		d.logger.Info("tailscale detected", "hostname", tailscale.DNSName, "ip", tailscale.IP)
	}

	// Note: Port forwarding is handled natively by the container's -p flag
	// No TCP proxy needed - the container CLI exposes ports directly on the host

	// Build PortMap from Nomad allocated ports
	// cfg.Resources.Ports contains the allocated port mappings for this task
	portMap := make(map[string]int)
	if cfg.Resources.Ports != nil {
		for _, port := range *cfg.Resources.Ports {
			portMap[port.Label] = port.Value
		}
	}
	if len(portMap) > 0 {
		d.logger.Info("built port map for driver network", "portmap", portMap)
	}

	// Build DriverNetwork with container IP for service registration
	// All services go through Traefik, which reaches containers directly on the styx network
	if containerIP != "" {
		driverNetwork = &drivers.DriverNetwork{
			IP:            containerIP,
			AutoAdvertise: true,
			PortMap:       portMap,
		}
		d.logger.Info("using container IP for service registration", "ip", containerIP)
	}

	// Store the handle
	d.tasksLock.Lock()
	d.tasks[cfg.ID] = handle
	d.tasksLock.Unlock()

	// Start monitoring the container
	go handle.run()

	// Build task state for recovery
	taskState := TaskState{
		TaskConfig:  &taskConfig,
		ContainerID: containerID,
		StartedAt:   handle.startedAt,
	}

	taskHandle := drivers.NewTaskHandle(taskHandleVersion)
	taskHandle.Config = cfg

	if err := taskHandle.SetDriverState(&taskState); err != nil {
		startErr = fmt.Errorf("failed to set driver state: %w", err)
		return nil, nil, startErr
	}

	return taskHandle, driverNetwork, nil
}

// RecoverTask attempts to recover a task handle from a previous driver instance.
// This is called during Nomad client restarts to reclaim running containers.
// It verifies the container still exists and recreates the task monitoring.
func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("handle is nil")
	}

	var taskState TaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		return fmt.Errorf("failed to decode task state: %w", err)
	}

	d.logger.Info("recovering task", "task_id", handle.Config.ID, "container_id", taskState.ContainerID)

	// Check if container still exists
	if !d.client.Exists(d.ctx, taskState.ContainerID) {
		return fmt.Errorf("container %s not found during recovery", taskState.ContainerID)
	}

	// Recreate handle
	h := newTaskHandle(d.client, d.logger, taskState.ContainerID, taskState.TaskConfig)
	h.startedAt = taskState.StartedAt

	d.tasksLock.Lock()
	d.tasks[handle.Config.ID] = h
	d.tasksLock.Unlock()

	go h.run()

	return nil
}

// WaitTask blocks until the task exits and returns the exit result.
// This is used by Nomad to monitor task completion and handle restarts.
func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	ch := make(chan *drivers.ExitResult)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		case <-handle.waitCh:
			ch <- handle.GetExitResult()
		}
	}()

	return ch, nil
}

// StopTask terminates a running container with the specified signal and timeout.
// If signal is empty, SIGTERM is used for graceful shutdown.
func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	d.logger.Info("stopping task", "task_id", taskID, "container_id", handle.containerID)

	// Send signal if specified
	if signal != "" {
		if err := d.client.Kill(d.ctx, handle.containerID, signal); err != nil {
			d.logger.Warn("failed to send signal", "error", err)
		}
	}

	// Stop the container
	ctx, cancel := context.WithTimeout(d.ctx, timeout)
	defer cancel()

	return d.client.Stop(ctx, handle.containerID)
}

// DestroyTask removes a container and cleans up associated resources.
// If force is true, the container is forcefully removed without graceful shutdown.
func (d *Driver) DestroyTask(taskID string, force bool) error {
	d.tasksLock.Lock()
	handle, ok := d.tasks[taskID]
	if !ok {
		d.tasksLock.Unlock()
		return nil
	}
	delete(d.tasks, taskID)
	d.tasksLock.Unlock()

	d.logger.Info("destroying task", "task_id", taskID, "container_id", handle.containerID)

	handle.shutdown()

	// Remove the container
	if err := d.client.Remove(d.ctx, handle.containerID); err != nil {
		d.logger.Warn("failed to remove container", "error", err)
	}

	return nil
}

// InspectTask returns the current status of a running task,
// including its state, start time, and exit result if completed.
func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return handle.TaskStatus(), nil
}

// TaskStats streams periodic resource usage statistics for a task.
// Uses the container CLI to fetch actual resource metrics for the running container.
func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	ch := make(chan *drivers.TaskResourceUsage)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats, err := d.client.Stats(ctx, handle.containerID)
				if err != nil {
					// Container may not be running, send empty stats
					ch <- &drivers.TaskResourceUsage{
						ResourceUsage: &drivers.ResourceUsage{
							MemoryStats: &drivers.MemoryStats{},
							CpuStats:    &drivers.CpuStats{},
						},
						Timestamp: time.Now().UnixNano(),
					}
					continue
				}

				ch <- &drivers.TaskResourceUsage{
					ResourceUsage: &drivers.ResourceUsage{
						MemoryStats: &drivers.MemoryStats{
							RSS:      stats.MemoryUsageBytes,
							MaxUsage: stats.MemoryLimitBytes,
						},
						CpuStats: &drivers.CpuStats{
							Percent: stats.CPUPercent,
						},
					},
					Timestamp: time.Now().UnixNano(),
				}
			}
		}
	}()

	return ch, nil
}

// TaskEvents returns a channel for receiving task lifecycle events
// such as task starting, stopping, or state changes.
func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

// SignalTask sends a Unix signal to a running task's container process.
// Common signals include SIGTERM, SIGKILL, SIGHUP, etc.
func (d *Driver) SignalTask(taskID string, signal string) error {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	return d.client.Kill(d.ctx, handle.containerID, signal)
}

// ExecTask executes a command inside a running container and returns the output.
// This is used for non-interactive command execution with a timeout.
func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	ctx, cancel := context.WithTimeout(d.ctx, timeout)
	defer cancel()

	output, err := d.client.Exec(ctx, handle.containerID, cmd)

	result := &drivers.ExecTaskResult{
		Stdout: output,
		Stderr: nil,
	}

	if err != nil {
		result.ExitResult = &drivers.ExitResult{
			ExitCode: 1,
			Err:      err,
		}
	} else {
		result.ExitResult = &drivers.ExitResult{
			ExitCode: 0,
		}
	}

	return result, nil
}

// ExecTaskStreaming executes an interactive command in a container with streaming I/O.
// This enables real-time stdin/stdout/stderr interaction for commands like shells.
func (d *Driver) ExecTaskStreaming(ctx context.Context, taskID string, opts *drivers.ExecOptions) (*drivers.ExitResult, error) {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	d.logger.Debug("exec streaming", "task_id", taskID, "container_id", handle.containerID, "command", opts.Command)

	err := d.client.ExecInteractive(ctx, handle.containerID, opts.Command, opts.Stdin, opts.Stdout, opts.Stderr)

	if err != nil {
		return &drivers.ExitResult{
			ExitCode: 1,
			Err:      err,
		}, nil
	}

	return &drivers.ExitResult{
		ExitCode: 0,
	}, nil
}

// Shutdown gracefully terminates the driver and cancels all background operations.
func (d *Driver) Shutdown() {
	d.cancel()
}
