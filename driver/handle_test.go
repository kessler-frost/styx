package main

import (
	"errors"
	"testing"

	"github.com/kessler-frost/styx/driver/container"
)

// TestClassifyExit is the regression test for the bug where a container's exit
// was ALWAYS reported as ExitCode 0 regardless of the real outcome, so a
// crashed container looked successful to Nomad.
func TestClassifyExit(t *testing.T) {
	tests := []struct {
		name         string
		info         *container.ContainerInfo
		inspectErr   error
		wantExitCode int
		wantErr      bool
	}{
		{
			name:         "clean stop",
			info:         &container.ContainerInfo{Status: "stopped"},
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "exited cleanly",
			info:         &container.ContainerInfo{Status: "exited"},
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "crashed / error state reported as failure",
			info:         &container.ContainerInfo{Status: "error"},
			wantExitCode: 1,
			wantErr:      true,
		},
		{
			name:         "unknown terminal state reported as failure",
			info:         &container.ContainerInfo{Status: "failed"},
			wantExitCode: 1,
			wantErr:      true,
		},
		{
			name:         "container uninspectable (vanished) reported as failure",
			info:         nil,
			inspectErr:   errors.New("container not found"),
			wantExitCode: -1,
			wantErr:      true,
		},
		{
			name:         "nil info without error still treated as failure",
			info:         nil,
			inspectErr:   nil,
			wantExitCode: -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyExit(tt.info, tt.inspectErr)
			if result == nil {
				t.Fatal("classifyExit returned nil")
			}
			if result.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExitCode)
			}
			if (result.Err != nil) != tt.wantErr {
				t.Errorf("Err = %v, wantErr = %v", result.Err, tt.wantErr)
			}
		})
	}
}

func TestIsRunningInfo(t *testing.T) {
	tests := []struct {
		name string
		info *container.ContainerInfo
		err  error
		want bool
	}{
		{"running", &container.ContainerInfo{Status: "running"}, nil, true},
		{"stopped", &container.ContainerInfo{Status: "stopped"}, nil, false},
		{"inspect error means not running", nil, errors.New("boom"), false},
		{"nil info means not running", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRunningInfo(tt.info, tt.err); got != tt.want {
				t.Errorf("isRunningInfo = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInspectStatus(t *testing.T) {
	if got := inspectStatus(nil, errors.New("x")); got != "uninspectable" {
		t.Errorf("inspectStatus(err) = %q, want uninspectable", got)
	}
	if got := inspectStatus(nil, nil); got != "unknown" {
		t.Errorf("inspectStatus(nil) = %q, want unknown", got)
	}
	if got := inspectStatus(&container.ContainerInfo{Status: "stopped"}, nil); got != "stopped" {
		t.Errorf("inspectStatus(stopped) = %q, want stopped", got)
	}
}
