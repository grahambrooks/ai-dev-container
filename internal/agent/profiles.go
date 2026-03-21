package agent

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
)

// Profile defines an AI agent's configuration for container setup.
type Profile struct {
	Name         string
	DisplayName  string
	Binary       string
	ConfigDirs   []string                          // Paths relative to home dir to mount
	MountMode    string                            // ro or rw
	NetworkAllow []string                          // Domains the agent needs
	Features     map[string]map[string]interface{} // Dev Container Features required
	InstallCmd   string                            // Shell command to install the agent in the container
	EnvVars      map[string]string                 // Environment variables to set in the container
}

var knownProfiles = map[string]*Profile{
	"claude": {
		Name:        "claude",
		DisplayName: "Claude Code",
		Binary:      "claude",
		ConfigDirs:  []string{".claude"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"api.anthropic.com",
			"statsig.anthropic.com",
			"sentry.io",
			"registry.npmjs.org",
		},
		Features: map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": {"version": "lts"},
		},
		InstallCmd: "npm install -g @anthropic-ai/claude-code",
		EnvVars:    map[string]string{},
	},
	"codex": {
		Name:        "codex",
		DisplayName: "OpenAI Codex CLI",
		Binary:      "codex",
		ConfigDirs:  []string{".codex"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"api.openai.com",
			"registry.npmjs.org",
		},
		Features: map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": {"version": "lts"},
		},
		InstallCmd: "npm install -g @openai/codex",
		EnvVars:    map[string]string{},
	},
	"gemini": {
		Name:        "gemini",
		DisplayName: "Gemini CLI",
		Binary:      "gemini",
		ConfigDirs:  []string{".gemini"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"generativelanguage.googleapis.com",
			"registry.npmjs.org",
		},
		Features: map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": {"version": "lts"},
		},
		InstallCmd: "npm install -g @anthropic-ai/gemini-cli || npm install -g @google/gemini-cli",
		EnvVars:    map[string]string{},
	},
	"aider": {
		Name:        "aider",
		DisplayName: "Aider",
		Binary:      "aider",
		ConfigDirs:  []string{".aider"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"api.anthropic.com",
			"api.openai.com",
			"pypi.org",
			"files.pythonhosted.org",
		},
		Features: map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/python:1": {"version": "3.12"},
		},
		InstallCmd: "pip install aider-chat",
		EnvVars:    map[string]string{},
	},
	"opencode": {
		Name:        "opencode",
		DisplayName: "Opencode",
		Binary:      "opencode",
		ConfigDirs:  []string{".opencode"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"api.anthropic.com",
			"api.openai.com",
			"registry.npmjs.org",
		},
		Features: map[string]map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": {"version": "lts"},
		},
		InstallCmd: "npm install -g opencode",
		EnvVars:    map[string]string{},
	},
}

// GetProfile returns the profile for a named agent.
func GetProfile(name string) *Profile {
	return knownProfiles[name]
}

// ListProfiles returns all known agent profile names sorted alphabetically.
func ListProfiles() []string {
	names := make([]string, 0, len(knownProfiles))
	for name := range knownProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// FormatProfileList returns a formatted table of available agent profiles.
func FormatProfileList() string {
	var s string
	for _, name := range ListProfiles() {
		p := knownProfiles[name]
		s += fmt.Sprintf("  %-12s %s\n", p.Name, p.DisplayName)
	}
	return s
}

// Detect checks which AI agents are installed on the host.
func Detect() []*Profile {
	var found []*Profile
	for _, name := range ListProfiles() {
		p := knownProfiles[name]
		if _, err := exec.LookPath(p.Binary); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// ConfigMounts returns Docker mount arguments for the agent's config directories.
func (p *Profile) ConfigMounts(homeDir string) []string {
	var mounts []string
	for _, dir := range p.ConfigDirs {
		src := filepath.Join(homeDir, dir)
		dst := filepath.Join("/home/dev", dir)
		mounts = append(mounts,
			"--mount", "type=bind,source="+src+",target="+dst+",readonly",
		)
	}
	return mounts
}

// DevcontainerFeatures returns the features map formatted for devcontainer.json.
func (p *Profile) DevcontainerFeatures() map[string]interface{} {
	features := make(map[string]interface{})
	for ref, opts := range p.Features {
		if len(opts) == 0 {
			features[ref] = map[string]interface{}{}
		} else {
			m := make(map[string]interface{})
			for k, v := range opts {
				m[k] = v
			}
			features[ref] = m
		}
	}
	return features
}
