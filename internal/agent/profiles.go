package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
)

// MountSpec describes a directory or file to make available in the container.
type MountSpec struct {
	HostPath      string // Path relative to home dir on host
	ContainerPath string // Absolute path in container (empty = mirror host relative to container user home)
	ReadOnly      bool   // Bind mount read-only from host
	Copy          bool   // Copy into container instead of mounting (container-local, writable, no host link)
}

// Profile defines an AI agent's configuration for container setup.
type Profile struct {
	Name           string
	DisplayName    string
	Binary         string
	ConfigMounts   []MountSpec       // Config/auth directories to mount
	NetworkAllow   []string          // Domains the agent needs
	InstallCmd     string            // Shell command to install the agent binary in the container
	EnvVars        map[string]string // Static environment variables
	EnvPassthrough []string          // Host env vars to forward into the container (e.g., API keys)
}

var knownProfiles = map[string]*Profile{
	"claude": {
		Name:        "claude",
		DisplayName: "Claude Code",
		Binary:      "claude",
		ConfigMounts: []MountSpec{
			{HostPath: ".claude/settings.json", Copy: true}, // User settings
			{HostPath: ".claude.json", ReadOnly: true},      // Global settings
		},
		NetworkAllow: []string{
			"api.anthropic.com",
			"statsig.anthropic.com",
			"sentry.io",
			"claude.ai",
		},
		InstallCmd: `set -e && ` +
			`curl -fsSL https://claude.ai/install.sh | bash`,
		EnvVars:        map[string]string{},
		EnvPassthrough: []string{"ANTHROPIC_API_KEY"},
	},
	"codex": {
		Name:        "codex",
		DisplayName: "OpenAI Codex CLI",
		Binary:      "codex",
		ConfigMounts: []MountSpec{
			{HostPath: ".codex", Copy: true},                 // Codex config and auth — seeded from host
			{HostPath: ".config/github-copilot", Copy: true}, // GitHub Copilot OAuth tokens — seeded from host
		},
		NetworkAllow: []string{
			"api.openai.com",
			"copilot-proxy.githubusercontent.com",
			"api.github.com",
			"github.com",
			"objects.githubusercontent.com",
		},
		InstallCmd: `set -e && ` +
			`mkdir -p ~/.local/bin && ` +
			`ARCH=$(uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/') && ` +
			`curl -fsSL "https://github.com/openai/codex/releases/latest/download/codex-linux-${ARCH}.tar.gz" | ` +
			`tar xz -C ~/.local/bin`,
		EnvVars:        map[string]string{},
		EnvPassthrough: []string{"OPENAI_API_KEY", "GITHUB_TOKEN"},
	},
	"copilot": {
		Name:        "copilot",
		DisplayName: "GitHub Copilot CLI",
		Binary:      "copilot",
		ConfigMounts: []MountSpec{
			{HostPath: ".config/github-copilot", Copy: true}, // OAuth tokens
			{HostPath: ".config/gh", Copy: true},             // gh CLI auth
		},
		NetworkAllow: []string{
			"copilot-proxy.githubusercontent.com",
			"api.github.com",
			"github.com",
			"objects.githubusercontent.com",
			"gh.io",
		},
		InstallCmd: `set -e && ` +
			`curl -fsSL https://gh.io/copilot-install | bash`,
		EnvVars:        map[string]string{},
		EnvPassthrough: []string{"GITHUB_TOKEN"},
	},
	"gemini": {
		Name:        "gemini",
		DisplayName: "Gemini CLI",
		Binary:      "gemini",
		ConfigMounts: []MountSpec{
			{HostPath: ".gemini", Copy: true},            // Gemini config and auth — seeded from host
			{HostPath: ".config/gcloud", ReadOnly: true}, // GCP credentials for ADC auth
		},
		NetworkAllow: []string{
			"generativelanguage.googleapis.com",
			"oauth2.googleapis.com",
			"accounts.google.com",
			"github.com",
			"objects.githubusercontent.com",
		},
		InstallCmd: `set -e && ` +
			`mkdir -p ~/.local/bin && ` +
			`ARCH=$(uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/') && ` +
			`curl -fsSL "https://github.com/google-gemini/gemini-cli/releases/latest/download/gemini-cli-linux-${ARCH}.tar.gz" | ` +
			`tar xz -C ~/.local/bin`,
		EnvVars:        map[string]string{},
		EnvPassthrough: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS"},
	},
	"aider": {
		Name:        "aider",
		DisplayName: "Aider",
		Binary:      "aider",
		ConfigMounts: []MountSpec{
			{HostPath: ".aider.conf.yml", ReadOnly: true}, // Aider config
			{HostPath: ".aider", ReadOnly: true},          // Aider data
		},
		NetworkAllow: []string{
			"api.anthropic.com",
			"api.openai.com",
			"github.com",
			"objects.githubusercontent.com",
		},
		InstallCmd: `set -e && ` +
			`curl -fsSL https://aider.chat/install.sh | sh`,
		EnvVars:        map[string]string{},
		EnvPassthrough: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"},
	},
	"opencode": {
		Name:        "opencode",
		DisplayName: "Opencode",
		Binary:      "opencode",
		ConfigMounts: []MountSpec{
			{HostPath: ".opencode", Copy: true}, // Auth state — seeded from host
		},
		NetworkAllow: []string{
			"api.anthropic.com",
			"api.openai.com",
			"github.com",
			"objects.githubusercontent.com",
		},
		InstallCmd: `set -e && ` +
			`mkdir -p ~/.local/bin && ` +
			`ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && ` +
			`curl -fsSL "https://github.com/opencodeco/opencode/releases/latest/download/opencode_linux_${ARCH}.tar.gz" | ` +
			`tar xz -C ~/.local/bin`,
		EnvVars:        map[string]string{},
		EnvPassthrough: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"},
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

// CommonAuthMounts returns read-only mounts for host authentication shared across all agents.
// Missing host paths are included here; the caller should skip them if the source doesn't exist.
func CommonAuthMounts() []MountSpec {
	mounts := []MountSpec{
		{HostPath: ".ssh", ReadOnly: true},
	}

	// Prefer ~/.gitconfig; fall back to XDG location
	home, _ := os.UserHomeDir()
	if home != "" {
		if _, err := os.Stat(filepath.Join(home, ".gitconfig")); err == nil {
			mounts = append(mounts, MountSpec{HostPath: ".gitconfig", ReadOnly: true})
		} else if _, err := os.Stat(filepath.Join(home, ".config", "git")); err == nil {
			mounts = append(mounts, MountSpec{HostPath: ".config/git", ReadOnly: true})
		}
	}

	return mounts
}

// SSHAuthSockMount returns the host and container socket paths for SSH agent forwarding.
// Returns empty strings if SSH_AUTH_SOCK is not set.
func SSHAuthSockMount() (hostSocket, containerSocket string) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return "", ""
	}

	if runtime.GOOS == "darwin" {
		// Docker Desktop for Mac provides a forwarding socket
		return "/run/host-services/ssh-auth.sock", "/run/host-services/ssh-auth.sock"
	}
	// Linux: bind the actual socket
	return sock, "/tmp/ssh-auth.sock"
}
