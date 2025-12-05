package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/kessler-frost/styx/driver/container"
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

// Driver is the Apple Container driver implementation
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

// NewDriver creates a new Apple Container driver
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

// PluginInfo returns information about the plugin
func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the schema for the driver configuration
func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig is called by Nomad to set the configuration
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

// TaskConfigSchema returns the schema for task configuration
func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

// Capabilities returns the capabilities of the driver
func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return driverCapabilities, nil
}

// Fingerprint streams health status of the driver
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

// StartTask starts a new task
func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if cfg.Resources == nil {
		return nil, nil, fmt.Errorf("task resources are required")
	}

	var taskConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&taskConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %w", err)
	}

	d.logger.Info("starting task", "task_id", cfg.ID, "image", taskConfig.Image)

	// Build run options
	opts := container.RunOptions{
		Name:       cfg.ID,
		Image:      taskConfig.Image,
		Command:    taskConfig.Command,
		Args:       taskConfig.Args,
		Env:        taskConfig.Env,
		Ports:      taskConfig.Ports,
		Volumes:    taskConfig.Volumes,
		Memory:     taskConfig.Memory,
		CPUs:       taskConfig.CPUs,
		WorkingDir: taskConfig.WorkingDir,
		Network:    taskConfig.Network,
		Detach:     true,
	}

	// Start the container
	containerID, err := d.client.Run(d.ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start container: %w", err)
	}

	d.logger.Info("container started", "container_id", containerID)

	// Create handle
	handle := newTaskHandle(d.client, d.logger, containerID, &taskConfig)

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
		return nil, nil, fmt.Errorf("failed to set driver state: %w", err)
	}

	return taskHandle, nil, nil
}

// RecoverTask restores a task from a previous driver state
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

// WaitTask returns a channel that will be closed when the task exits
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

// StopTask stops a running task
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

// DestroyTask removes a task
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

// InspectTask returns status of a task
func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return handle.TaskStatus(), nil
}

// TaskStats returns resource usage stats for a task
func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	d.tasksLock.RLock()
	_, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// For now, return empty stats - can be enhanced later with actual resource monitoring
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
				ch <- &drivers.TaskResourceUsage{
					ResourceUsage: &drivers.ResourceUsage{
						MemoryStats: &drivers.MemoryStats{},
						CpuStats:    &drivers.CpuStats{},
					},
					Timestamp: time.Now().UnixNano(),
				}
			}
		}
	}()

	return ch, nil
}

// TaskEvents returns a channel for task events
func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

// SignalTask sends a signal to a task
func (d *Driver) SignalTask(taskID string, signal string) error {
	d.tasksLock.RLock()
	handle, ok := d.tasks[taskID]
	d.tasksLock.RUnlock()

	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	return d.client.Kill(d.ctx, handle.containerID, signal)
}

// ExecTask runs a command in a task
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

// Shutdown cleans up the driver
func (d *Driver) Shutdown() {
	d.cancel()
}
