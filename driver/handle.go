package main

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/kessler-frost/styx/driver/container"
)

// taskHandle tracks a running container
type taskHandle struct {
	containerID string
	client      *container.Client
	logger      hclog.Logger

	// stateLock synchronizes access to procState
	stateLock sync.RWMutex

	// taskConfig contains the task configuration
	taskConfig *TaskConfig

	// startedAt is the time the container was started
	startedAt time.Time

	// exitResult stores the exit result once the container exits
	exitResult *drivers.ExitResult

	// waitCh is used to wait for the container to exit
	waitCh chan struct{}

	// ctx and cancel for the handle
	ctx    context.Context
	cancel context.CancelFunc
}

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

// run monitors the container and waits for it to exit
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

// shutdown stops monitoring the container
func (h *taskHandle) shutdown() {
	h.cancel()
}

// IsRunning returns whether the container is running
func (h *taskHandle) IsRunning() bool {
	return h.client.IsRunning(h.ctx, h.containerID)
}

// TaskStatus returns the status of the task
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

// GetExitResult returns the exit result of the container
func (h *taskHandle) GetExitResult() *drivers.ExitResult {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()
	return h.exitResult
}
