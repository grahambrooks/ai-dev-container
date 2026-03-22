package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/pkg/types"
)

// configSnapshot captures the fields that affect container build/setup.
// A change to any of these means the container should be rebuilt.
type configSnapshot struct {
	Image             string                 `json:"image"`
	Features          map[string]interface{} `json:"features,omitempty"`
	Agent             string                 `json:"agent,omitempty"`
	SecurityProfile   string                 `json:"securityProfile,omitempty"`
	PostCreateCommand interface{}            `json:"postCreateCommand,omitempty"`
	OnCreateCommand   interface{}            `json:"onCreateCommand,omitempty"`
	ContainerEnv      map[string]string      `json:"containerEnv,omitempty"`
	EnvPassthrough    []string               `json:"envPassthrough,omitempty"`
	ResourcesCPUs     string                 `json:"cpus,omitempty"`
	ResourcesMemory   string                 `json:"memory,omitempty"`
	NetworkMode       string                 `json:"networkMode,omitempty"`
	AgentMounts       []mountSnapshot        `json:"agentMounts,omitempty"`
}

type mountSnapshot struct {
	HostPath string `json:"hostPath"`
	ReadOnly bool   `json:"readOnly"`
	Seed     bool   `json:"seed"`
}

// ConfigHash computes a hash of all config fields that affect how the container
// is built and configured. Two configs with the same hash produce identical containers.
func ConfigHash(devCfg *types.DevContainerConfig, custom *types.DevcCustomization) string {
	snap := configSnapshot{
		Image:             devCfg.Image,
		Features:          devCfg.Features,
		Agent:             custom.Agent,
		SecurityProfile:   custom.SecurityProfile,
		PostCreateCommand: devCfg.PostCreateCommand,
		OnCreateCommand:   devCfg.OnCreateCommand,
		ContainerEnv:      devCfg.ContainerEnv,
	}

	if custom.EnvPassthrough != nil {
		sorted := make([]string, len(custom.EnvPassthrough))
		copy(sorted, custom.EnvPassthrough)
		sort.Strings(sorted)
		snap.EnvPassthrough = sorted
	}
	if custom.Resources != nil {
		snap.ResourcesCPUs = custom.Resources.CPUs
		snap.ResourcesMemory = custom.Resources.Memory
	}
	if custom.Network != nil {
		snap.NetworkMode = custom.Network.Mode
	}

	// Include agent mount specs so changes to mount modes trigger rebuild
	if custom.Agent != "" {
		if profile := agent.GetProfile(custom.Agent); profile != nil {
			for _, m := range profile.ConfigMounts {
				snap.AgentMounts = append(snap.AgentMounts, mountSnapshot{
					HostPath: m.HostPath,
					ReadOnly: m.ReadOnly,
					Seed:     m.Seed,
				})
			}
		}
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return "unknown"
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
