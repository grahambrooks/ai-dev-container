package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grahambrooks/devc/pkg/types"
)

const (
	DevcDir        = ".devc"
	ConfigFileName = "config.json"
)

// LoadDevcontainerConfig finds and parses devcontainer.json from the workspace.
func LoadDevcontainerConfig(workspaceFolder string) (*types.DevContainerConfig, error) {
	paths := []string{
		filepath.Join(workspaceFolder, ".devcontainer", "devcontainer.json"),
		filepath.Join(workspaceFolder, ".devcontainer.json"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg types.DevContainerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}
		return &cfg, nil
	}

	return nil, fmt.Errorf("no devcontainer.json found in %s", workspaceFolder)
}

// LoadGlobalConfig reads ~/.devc/config.json.
func LoadGlobalConfig() (*types.GlobalConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultGlobalConfig(), nil
	}

	p := filepath.Join(home, DevcDir, ConfigFileName)
	data, err := os.ReadFile(p)
	if err != nil {
		return defaultGlobalConfig(), nil
	}

	var cfg types.GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	return &cfg, nil
}

// ExtractDevcCustomization pulls the devc-specific customization from devcontainer.json.
func ExtractDevcCustomization(cfg *types.DevContainerConfig) (*types.DevcCustomization, error) {
	if cfg.Customizations == nil {
		return defaultDevcCustomization(), nil
	}

	raw, ok := cfg.Customizations["devc"]
	if !ok {
		return defaultDevcCustomization(), nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var custom types.DevcCustomization
	if err := json.Unmarshal(data, &custom); err != nil {
		return nil, fmt.Errorf("parsing devc customization: %w", err)
	}
	return &custom, nil
}

// MergeCustomization merges global defaults with project-level customization.
// Project values take precedence over global defaults.
func MergeCustomization(global *types.GlobalConfig, project *types.DevcCustomization) *types.DevcCustomization {
	merged := global.Defaults

	if project.Agent != "" {
		merged.Agent = project.Agent
	}
	if project.SecurityProfile != "" {
		merged.SecurityProfile = project.SecurityProfile
	}
	if project.Network != nil {
		merged.Network = project.Network
	}
	if project.Resources != nil {
		merged.Resources = project.Resources
	}
	if project.Filesystem != nil {
		merged.Filesystem = project.Filesystem
	}
	if project.Session != nil {
		merged.Session = project.Session
	}
	if project.AgentMounts != nil {
		merged.AgentMounts = project.AgentMounts
	}

	return &merged
}

// ContainerName generates a deterministic container name from the workspace path.
func ContainerName(workspaceFolder string) string {
	abs, err := filepath.Abs(workspaceFolder)
	if err != nil {
		abs = workspaceFolder
	}

	base := filepath.Base(abs)
	hash := sha256.Sum256([]byte(abs))
	short := fmt.Sprintf("%x", hash[:4])

	// Sanitize base name for Docker container naming
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, base)

	return fmt.Sprintf("devc-%s-%s", safe, short)
}

// WorkspaceInContainer returns the workspace mount target path inside the container.
func WorkspaceInContainer(cfg *types.DevContainerConfig, workspaceFolder string) string {
	if cfg.WorkspaceFolder != "" {
		return cfg.WorkspaceFolder
	}
	return filepath.Join("/workspaces", filepath.Base(workspaceFolder))
}

func defaultGlobalConfig() *types.GlobalConfig {
	return &types.GlobalConfig{
		Defaults: *defaultDevcCustomization(),
	}
}

func defaultDevcCustomization() *types.DevcCustomization {
	return &types.DevcCustomization{
		SecurityProfile: "moderate",
		Network: &types.NetworkConfig{
			Mode: "restricted",
		},
		Resources: &types.ResourceConfig{
			CPUs:      "4",
			Memory:    "8g",
			PidsLimit: 256,
		},
		Filesystem: &types.FilesystemConfig{
			ProjectMountMode: "rw",
		},
		Session: &types.SessionConfig{
			StopOnLastDetach: true,
		},
	}
}
