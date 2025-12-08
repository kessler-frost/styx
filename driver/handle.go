package main

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/kessler-frost/styx/driver/container"
)

// taskHandle manages the lifecycle of a running container task.
// It tracks the container process and provides methods for stopping and cleanup.
type taskHandle struct {
	// containerID is the unique identifier for the running container.
	containerID string

	// client is the container CLI client used to interact with the container.
	client *container.Client

	// logger is the structured logger for this task handle.
	logger hclog.Logger

	// stateLock synchronizes access to exitResult.
	stateLock sync.RWMutex

	// taskConfig contains the original task configuration.
	taskConfig *TaskConfig

	// startedAt is the timestamp when the container was started.
	startedAt time.Time

	// exitResult stores the exit result once the container exits.
	exitResult *drivers.ExitResult

	// waitCh is closed when the container exits.
	waitCh chan struct{}

	// ctx and cancel control the handle's background operations.
	ctx    context.Context
	cancel context.CancelFunc
}

// newTaskHandle creates a new task handle for managing a container's lifecycle.
// It initializes monitoring for the container and prepares exit result tracking.
func newTaskHandle(client *container.Client, logger hclog.Logger, containerID string, taskConfig *TaskConfig) *taskHandle {
	ctx, cancel := context.WithCancel(context.Background())
	h := &taskHandle{
		containerID: containerID,
		client:      client,
		logger:      logger.Named("handle").With("container_id", containerID),
		taskConfig:  taskConfig,
		startedAt:   time.Now(),
		waitCh:      make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
	}
	return h
}

// run monitors the container status and waits for it to exit.
// It periodically polls the container state and closes waitCh when the container exits.
func (h *taskHandle) run() {
	defer close(h.waitCh)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			running := h.client.IsRunning(h.ctx, h.containerID)
			if !running {
				h.stateLock.Lock()
				h.exitResult = &drivers.ExitResult{
					ExitCode:  0,
					Signal:    0,
					OOMKilled: false,
					Err:       nil,
				}
				h.stateLock.Unlock()
				h.logger.Info("container exited")
				return
			}
		}
	}
}

// shutdown stops monitoring the container and cancels all background operations.
func (h *taskHandle) shutdown() {
	h.cancel()
}

// IsRunning checks if the container is currently running.
func (h *taskHandle) IsRunning() bool {
	return h.client.IsRunning(h.ctx, h.containerID)
}

// TaskStatus returns the current status of the task including its state,
// start time, completion time, and exit result if the task has exited.
func (h *taskHandle) TaskStatus() *drivers.TaskStatus {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()

	status := &drivers.TaskStatus{
		ID:          h.containerID,
		Name:        h.taskConfig.Image,
		StartedAt:   h.startedAt,
		CompletedAt: time.Time{},
		ExitResult:  nil,
		State:       drivers.TaskStateRunning,
	}

	if h.exitResult != nil {
		status.CompletedAt = time.Now()
		status.State = drivers.TaskStateExited
		status.ExitResult = h.exitResult
	}

	return status
}

// GetExitResult returns the exit result of the container.
// Returns nil if the container is still running.
func (h *taskHandle) GetExitResult() *drivers.ExitResult {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()
	return h.exitResult
}
