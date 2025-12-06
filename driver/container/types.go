package container

// ContainerInfo represents the JSON output from `container list` and `container inspect`
type ContainerInfo struct {
	Status        string        `json:"status"`
	Configuration Configuration `json:"configuration"`
	Networks      []NetworkInfo `json:"networks"`
}

type Configuration struct {
	ID               string           `json:"id"`
	Image            ImageInfo        `json:"image"`
	Resources        Resources        `json:"resources"`
	Platform         Platform         `json:"platform"`
	InitProcess      InitProcess      `json:"initProcess"`
	DNS              DNS              `json:"dns"`
	RuntimeHandler   string           `json:"runtimeHandler"`
	SSH              bool             `json:"ssh"`
	Rosetta          bool             `json:"rosetta"`
	Virtualization   bool             `json:"virtualization"`
	Labels           map[string]string `json:"labels"`
	Mounts           []Mount          `json:"mounts"`
	PublishedPorts   []PublishedPort  `json:"publishedPorts"`
	PublishedSockets []PublishedSocket `json:"publishedSockets"`
	Networks         []NetworkConfig  `json:"networks"`
	Sysctls          map[string]string `json:"sysctls"`
}

type ImageInfo struct {
	Reference  string          `json:"reference"`
	Descriptor ImageDescriptor `json:"descriptor"`
}

type ImageDescriptor struct {
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

type Resources struct {
	MemoryInBytes int64 `json:"memoryInBytes"`
	CPUs          int   `json:"cpus"`
}

type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type InitProcess struct {
	WorkingDirectory   string   `json:"workingDirectory"`
	Executable         string   `json:"executable"`
	Arguments          []string `json:"arguments"`
	Environment        []string `json:"environment"`
	Terminal           bool     `json:"terminal"`
	User               User     `json:"user"`
	Rlimits            []Rlimit `json:"rlimits"`
	SupplementalGroups []int    `json:"supplementalGroups"`
}

type User struct {
	ID UserID `json:"id"`
}

type UserID struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

type Rlimit struct {
	Type string `json:"type"`
	Hard uint64 `json:"hard"`
	Soft uint64 `json:"soft"`
}

type DNS struct {
	Nameservers   []string `json:"nameservers"`
	SearchDomains []string `json:"searchDomains"`
	Options       []string `json:"options"`
}

type Mount struct {
	Type        interface{} `json:"type"`        // Can be object like {"virtiofs":{}} or string
	Source      string      `json:"source"`
	Destination string      `json:"destination"` // Apple container uses "destination", not "target"
	Options     []string    `json:"options"`
}

type PublishedPort struct {
	HostIP        string `json:"hostIP,omitempty"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

type PublishedSocket struct {
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
}

type NetworkConfig struct {
	Network string                 `json:"network"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type NetworkInfo struct {
	Network  string `json:"network"`
	Address  string `json:"address"`
	Gateway  string `json:"gateway"`
	Hostname string `json:"hostname"`
}

// RunOptions contains options for running a container
type RunOptions struct {
	Name       string
	Image      string
	Command    string
	Args       []string
	Env        map[string]string
	Ports      []string
	Volumes    []string
	Memory     string
	CPUs       int
	Detach     bool
	Remove     bool
	Network    string
	WorkingDir string
}
