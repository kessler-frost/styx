package main

import (
	"time"

	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

var (
	// configSpec is the HCL specification for the driver configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"container_bin_path": hclspec.NewDefault(
			hclspec.NewAttr("container_bin_path", "string", false),
			hclspec.NewLiteral(`""`),
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

// Config contains the plugin-level configuration for the Apple Container driver.
// This includes the path to the container binary and other driver-wide settings.
type Config struct {
	// ContainerBinPath is the path to the Apple container CLI binary.
	// If empty, the driver will look up "container" in PATH.
	ContainerBinPath string `codec:"container_bin_path"`
}

// TaskConfig contains the driver-specific configuration for running a container.
// This includes the container image, port mappings, volumes, and environment variables.
type TaskConfig struct {
	// Image is the OCI container image to run (required).
	Image string `codec:"image"`

	// Command overrides the default command in the container image.
	Command string `codec:"command"`

	// Args are additional arguments passed to the container command.
	Args []string `codec:"args"`

	// Env is a map of environment variables to set in the container.
	Env hclutils.MapStrStr `codec:"env"`

	// Ports are port mappings in the format "host:container" or "container".
	Ports []string `codec:"ports"`

	// Volumes are volume mounts. Supports two formats:
	// - Bind mount: "/host/path:/container/path" or "/host/path:/container/path:ro"
	// - Named volume: "volume-name:/container/path" (auto-created if doesn't exist)
	// The driver automatically mounts /local, /secrets, and /alloc directories.
	Volumes []string `codec:"volumes"`

	// Memory is the memory limit for the container (e.g., "512m", "2g").
	Memory string `codec:"memory"`

	// CPUs is the number of CPUs to allocate to the container.
	CPUs int `codec:"cpus"`

	// WorkingDir sets the working directory inside the container.
	WorkingDir string `codec:"working_dir"`

	// Network specifies which container network to use.
	// Defaults to the "styx" network for container-to-container communication.
	Network string `codec:"network"`
}

// TaskState is the state which is encoded in the handle returned to Nomad client.
// This information is needed to rebuild the task state and handler during
// temporary unavailability of the driver (e.g., when the driver is upgraded).
type TaskState struct {
	// TaskConfig is the original task configuration.
	TaskConfig *TaskConfig

	// ContainerID is the unique identifier for the running container.
	ContainerID string

	// StartedAt is the timestamp when the container was started.
	StartedAt time.Time
}
