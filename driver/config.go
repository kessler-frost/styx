package main

import (
	"time"

	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

var (
	// configSpec is the HCL specification for the driver configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"container_bin_path": hclspec.NewDefault(
			hclspec.NewAttr("container_bin_path", "string", false),
			hclspec.NewLiteral(`"/usr/local/bin/container"`),
		),
	})

	// taskConfigSpec is the HCL specification for the task configuration
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image": hclspec.NewAttr("image", "string", true),
		"command": hclspec.NewAttr("command", "string", false),
		"args": hclspec.NewAttr("args", "list(string)", false),
		"env": hclspec.NewAttr("env", "list(map(string))", false),
		"ports": hclspec.NewAttr("ports", "list(string)", false),
		"volumes": hclspec.NewAttr("volumes", "list(string)", false),
		"memory": hclspec.NewAttr("memory", "string", false),
		"cpus": hclspec.NewAttr("cpus", "number", false),
		"working_dir": hclspec.NewAttr("working_dir", "string", false),
		"network": hclspec.NewAttr("network", "string", false),
	})
)

// Config contains the driver configuration
type Config struct {
	ContainerBinPath string `codec:"container_bin_path"`
}

// TaskConfig contains the task configuration
type TaskConfig struct {
	Image      string            `codec:"image"`
	Command    string            `codec:"command"`
	Args       []string          `codec:"args"`
	Env        map[string]string `codec:"env"`
	Ports      []string          `codec:"ports"`
	Volumes    []string          `codec:"volumes"`
	Memory     string            `codec:"memory"`
	CPUs       int               `codec:"cpus"`
	WorkingDir string            `codec:"working_dir"`
	Network    string            `codec:"network"`
}

// TaskState is the state which is encoded in the handle returned to Nomad client.
// This information is needed to rebuild the task state and handler during
// temporary unavailability of the driver (e.g., when the driver is upgraded).
type TaskState struct {
	TaskConfig  *TaskConfig
	ContainerID string
	StartedAt   time.Time
}
