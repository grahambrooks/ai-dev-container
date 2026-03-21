package agent

import (
	"os/exec"
	"path/filepath"
)

// Profile defines an AI agent's configuration for container setup.
type Profile struct {
	Name         string
	DisplayName  string
	Binary       string
	ConfigDirs   []string // Paths relative to home dir to mount
	MountMode    string   // ro or rw
	NetworkAllow []string // Domains the agent needs
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
		},
	},
	"codex": {
		Name:        "codex",
		DisplayName: "OpenAI Codex",
		Binary:      "codex",
		ConfigDirs:  []string{".codex"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"api.openai.com",
		},
	},
	"gemini": {
		Name:        "gemini",
		DisplayName: "Gemini CLI",
		Binary:      "gemini",
		ConfigDirs:  []string{".gemini"},
		MountMode:   "ro",
		NetworkAllow: []string{
			"generativelanguage.googleapis.com",
		},
	},
	"opencode": {
		Name:        "opencode",
		DisplayName: "Opencode",
		Binary:      "opencode",
		ConfigDirs:  []string{".opencode"},
		MountMode:   "ro",
		NetworkAllow: []string{},
	},
}

// GetProfile returns the profile for a named agent.
func GetProfile(name string) *Profile {
	return knownProfiles[name]
}

// ListProfiles returns all known agent profile names.
func ListProfiles() []string {
	names := make([]string, 0, len(knownProfiles))
	for name := range knownProfiles {
		names = append(names, name)
	}
	return names
}

// Detect checks which AI agents are installed on the host.
func Detect() []*Profile {
	var found []*Profile
	for _, p := range knownProfiles {
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
