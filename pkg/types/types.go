package types

// DevContainerConfig represents a parsed devcontainer.json.
type DevContainerConfig struct {
	Name                 string                 `json:"name,omitempty"`
	Image                string                 `json:"image,omitempty"`
	Build                *BuildConfig           `json:"build,omitempty"`
	DockerComposeFile    interface{}            `json:"dockerComposeFile,omitempty"`
	Service              string                 `json:"service,omitempty"`
	RunArgs              []string               `json:"runArgs,omitempty"`
	ContainerEnv         map[string]string      `json:"containerEnv,omitempty"`
	RemoteEnv            map[string]string      `json:"remoteEnv,omitempty"`
	RemoteUser           string                 `json:"remoteUser,omitempty"`
	Mounts               []interface{}          `json:"mounts,omitempty"`
	Features             map[string]interface{} `json:"features,omitempty"`
	ForwardPorts         []interface{}          `json:"forwardPorts,omitempty"`
	PostCreateCommand    interface{}            `json:"postCreateCommand,omitempty"`
	PostStartCommand     interface{}            `json:"postStartCommand,omitempty"`
	PostAttachCommand    interface{}            `json:"postAttachCommand,omitempty"`
	InitializeCommand    interface{}            `json:"initializeCommand,omitempty"`
	OnCreateCommand      interface{}            `json:"onCreateCommand,omitempty"`
	UpdateContentCommand interface{}            `json:"updateContentCommand,omitempty"`
	Customizations       map[string]interface{} `json:"customizations,omitempty"`
	OverrideCommand      *bool                  `json:"overrideCommand,omitempty"`
	ShutdownAction       string                 `json:"shutdownAction,omitempty"`
	WorkspaceFolder      string                 `json:"workspaceFolder,omitempty"`
	WorkspaceMount       string                 `json:"workspaceMount,omitempty"`
}

// BuildConfig holds Dockerfile build settings.
type BuildConfig struct {
	Dockerfile string            `json:"dockerfile,omitempty"`
	Context    string            `json:"context,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
	Target     string            `json:"target,omitempty"`
	CacheFrom  interface{}       `json:"cacheFrom,omitempty"`
}

// DevcCustomization holds AI safety extensions under customizations.devc.
type DevcCustomization struct {
	Agent           string            `json:"agent,omitempty"`
	SecurityProfile string            `json:"securityProfile,omitempty"`
	Network         *NetworkConfig    `json:"network,omitempty"`
	Resources       *ResourceConfig   `json:"resources,omitempty"`
	Filesystem      *FilesystemConfig `json:"filesystem,omitempty"`
	Session         *SessionConfig    `json:"session,omitempty"`
	AgentMounts     map[string]string `json:"agentMounts,omitempty"`
	EnvPassthrough  []string          `json:"envPassthrough,omitempty"` // Host env vars to forward (e.g., API keys)
}

type NetworkConfig struct {
	Mode      string   `json:"mode,omitempty"` // none, restricted, host
	Allowlist []string `json:"allowlist,omitempty"`
	Denylist  []string `json:"denylist,omitempty"`
}

type ResourceConfig struct {
	CPUs      string `json:"cpus,omitempty"`
	Memory    string `json:"memory,omitempty"`
	PidsLimit int64  `json:"pidsLimit,omitempty"`
}

type FilesystemConfig struct {
	ReadOnlyPaths    []string `json:"readOnlyPaths,omitempty"`
	NoExecPaths      []string `json:"noExecPaths,omitempty"`
	ProjectMountMode string   `json:"projectMountMode,omitempty"` // rw, ro, overlay
}

type SessionConfig struct {
	StopOnLastDetach   bool `json:"stopOnLastDetach,omitempty"`
	IdleTimeoutMinutes int  `json:"idleTimeoutMinutes,omitempty"`
}

// GlobalConfig represents ~/.devc/config.json.
type GlobalConfig struct {
	Defaults DevcCustomization      `json:"defaults"`
	Agents   map[string]AgentConfig `json:"agents,omitempty"`
}

type AgentConfig struct {
	ConfigPaths []string `json:"configPaths,omitempty"`
	MountMode   string   `json:"mountMode,omitempty"`
}

// ContainerInfo holds runtime info about a managed container.
type ContainerInfo struct {
	Name            string `json:"name"`
	ContainerID     string `json:"containerId"`
	WorkspaceFolder string `json:"workspaceFolder"`
	State           string `json:"state"`
	Image           string `json:"image"`
	Agent           string `json:"agent,omitempty"`
	Sessions        int    `json:"sessions"`
}

// SecurityProfile defines container security constraints.
type SecurityProfile struct {
	Name           string
	Network        NetworkConfig
	Resources      ResourceConfig
	DropAllCaps    bool
	AddCaps        []string
	SeccompProfile string
	RunAsUser      string
}
