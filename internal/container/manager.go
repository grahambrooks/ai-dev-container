package container

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/internal/config"
	"github.com/grahambrooks/devc/internal/docker"
	"github.com/grahambrooks/devc/internal/security"
	"github.com/grahambrooks/devc/internal/session"
	"github.com/grahambrooks/devc/pkg/types"
)

// Manager orchestrates container lifecycle operations.
type Manager struct {
	Docker  *docker.Client
	Session *session.Tracker
}

// NewManager creates a container manager.
func NewManager() (*Manager, error) {
	dc, err := docker.NewClient()
	if err != nil {
		return nil, err
	}

	tracker, err := session.NewTracker()
	if err != nil {
		return nil, err
	}

	return &Manager{Docker: dc, Session: tracker}, nil
}

// UpOptions configures the "up" command.
type UpOptions struct {
	WorkspaceFolder string
	Agent           string
	SecurityProfile string
	Detach          bool
	Rebuild         bool // Force rebuild even if container exists
}

// Up creates or starts a container for the workspace.
func (m *Manager) Up(opts UpOptions) error {
	devCfg, err := config.LoadDevcontainerConfig(opts.WorkspaceFolder)
	if err != nil {
		return err
	}

	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return err
	}

	custom, err := config.ExtractDevcCustomization(devCfg)
	if err != nil {
		return err
	}

	// CLI overrides
	if opts.Agent != "" {
		custom.Agent = opts.Agent
	}
	if opts.SecurityProfile != "" {
		custom.SecurityProfile = opts.SecurityProfile
	}

	merged := config.MergeCustomization(globalCfg, custom)

	containerName := config.ContainerName(opts.WorkspaceFolder)

	// Resolve agent profile
	var agentProfile *agent.Profile
	if merged.Agent != "" {
		agentProfile = agent.GetProfile(merged.Agent)
		if agentProfile == nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: unknown agent %q, skipping agent configuration\n", merged.Agent)
		}
	}

	// Compute config hash for drift detection
	currentHash := config.ConfigHash(devCfg, merged)

	// Check existing container state
	inspectResult := m.Docker.Inspect(containerName)
	state := inspectResult.State

	// Detect config drift on existing containers
	if !opts.Rebuild && (state == docker.StateRunning || state == docker.StateStopped || state == docker.StateCreated) {
		storedHash := inspectResult.Labels["devc.config-hash"]
		if storedHash != "" && storedHash != currentHash {
			changes := describeChanges(inspectResult.Labels, devCfg, merged, agentProfile)
			fmt.Printf("Configuration has changed since this container was created:\n")
			for _, change := range changes {
				fmt.Printf("  - %s\n", change)
			}
			fmt.Printf("\nRebuild container? [y/N] ")
			if askYesNo() {
				opts.Rebuild = true
			} else {
				fmt.Println("Continuing with existing container")
			}
		}
	}

	// Rebuild: remove existing container first
	if opts.Rebuild && state != docker.StateNotFound {
		fmt.Printf("Removing existing container %s...\n", containerName)
		if err := m.Docker.Remove(containerName, true); err != nil {
			return fmt.Errorf("removing container for rebuild: %w", err)
		}
		m.Session.Clean(containerName)
		state = docker.StateNotFound
	}

	switch state {
	case docker.StateRunning:
		fmt.Printf("Container %s is already running\n", containerName)

	case docker.StateStopped, docker.StateCreated:
		fmt.Printf("Starting existing container %s...\n", containerName)
		if err := m.Docker.Start(containerName); err != nil {
			return fmt.Errorf("starting container: %w", err)
		}

	case docker.StateNotFound:
		if err := m.createContainer(containerName, devCfg, merged, opts.WorkspaceFolder, agentProfile, currentHash); err != nil {
			return err
		}
	}

	// Track session
	count, _ := m.Session.Attach(containerName)
	fmt.Printf("Container %s ready (%s)\n", containerName, session.FormatCount(count))

	if !opts.Detach {
		return m.Docker.Exec(containerName, []string{"/bin/bash"}, true)
	}

	return nil
}

// createContainer handles image pull, feature build, container creation, and lifecycle commands.
func (m *Manager) createContainer(
	containerName string,
	devCfg *types.DevContainerConfig,
	custom *types.DevcCustomization,
	workspaceFolder string,
	agentProfile *agent.Profile,
	configHash string,
) error {
	// Pull base image if needed
	if devCfg.Image != "" && !m.Docker.ImageExists(devCfg.Image) {
		fmt.Printf("Pulling image %s...\n", devCfg.Image)
		if err := m.Docker.Pull(devCfg.Image); err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
	}

	// Build image with features if any are configured
	effectiveImage := devCfg.Image
	if len(devCfg.Features) > 0 {
		built, buildErr := m.Docker.BuildImageWithFeatures(devCfg.Image, devCfg.Features, containerName)
		if buildErr != nil {
			return fmt.Errorf("building image with features: %w", buildErr)
		}
		effectiveImage = built
	}

	// Swap the image for container creation
	origImage := devCfg.Image
	devCfg.Image = effectiveImage

	fmt.Printf("Creating container %s...\n", containerName)
	if err := m.Docker.CreateAndStart(containerName, devCfg, custom, workspaceFolder, agentProfile, configHash); err != nil {
		devCfg.Image = origImage
		return fmt.Errorf("creating container: %w", err)
	}
	devCfg.Image = origImage

	// Resolve container user's home directory for agent setup
	secProfile := security.GetProfile(custom.SecurityProfile)
	containerHome := m.Docker.ResolveHomeDir(effectiveImage, secProfile.RunAsUser)

	// Copy agent config from host into container (container-local, writable)
	if agentProfile != nil {
		m.copyAgentConfig(containerName, agentProfile, containerHome)
	}

	// Set up agent workspace path mappings (e.g., Claude trust folders)
	if agentProfile != nil {
		wsInContainer := config.WorkspaceInContainer(devCfg, workspaceFolder)
		m.setupAgentPathMappings(containerName, agentProfile, workspaceFolder, wsInContainer)
	}

	// Run lifecycle commands in order
	if devCfg.OnCreateCommand != nil {
		if lcErr := m.runLifecycleCommand(containerName, devCfg.OnCreateCommand, "onCreateCommand"); lcErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: onCreateCommand failed: %v\n", lcErr)
		}
	}
	if devCfg.PostCreateCommand != nil {
		if lcErr := m.runLifecycleCommand(containerName, devCfg.PostCreateCommand, "postCreateCommand"); lcErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: postCreateCommand failed: %v\n", lcErr)
		}
	}
	if devCfg.PostStartCommand != nil {
		if lcErr := m.runLifecycleCommand(containerName, devCfg.PostStartCommand, "postStartCommand"); lcErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: postStartCommand failed: %v\n", lcErr)
		}
	}

	// Ensure agent binary is on PATH by symlinking from ~/.local/bin into /usr/local/bin
	if agentProfile != nil {
		m.linkAgentBinary(containerName, agentProfile)
	}

	return nil
}

// Exec runs a command in the container for the given workspace.
func (m *Manager) Exec(workspaceFolder string, command []string) error {
	containerName := config.ContainerName(workspaceFolder)

	state := m.Docker.Inspect(containerName).State
	if state != docker.StateRunning {
		return fmt.Errorf("container %s is not running (state: %s)", containerName, state)
	}

	return m.Docker.ExecAs(containerName, command, docker.ExecOptions{
		Interactive: true,
	})
}

// Attach attaches an interactive session to the container.
func (m *Manager) Attach(workspaceFolder, shell string) error {
	containerName := config.ContainerName(workspaceFolder)

	state := m.Docker.Inspect(containerName).State
	if state != docker.StateRunning {
		return fmt.Errorf("container %s is not running", containerName)
	}

	count, _ := m.Session.Attach(containerName)
	fmt.Printf("Attached (%s)\n", session.FormatCount(count))

	err := m.Docker.Exec(containerName, []string{shell}, true)

	remaining, _ := m.Session.Detach(containerName)
	fmt.Printf("Detached (%s)\n", session.FormatCount(remaining))

	return err
}

// Stop stops the container, respecting session count.
func (m *Manager) Stop(workspaceFolder string, force bool) error {
	containerName := config.ContainerName(workspaceFolder)

	state := m.Docker.Inspect(containerName).State
	if state != docker.StateRunning {
		fmt.Printf("Container %s is not running\n", containerName)
		return nil
	}

	if !force {
		count := m.Session.Count(containerName)
		if count > 0 {
			return fmt.Errorf("container %s has %s; use --force to stop anyway", containerName, session.FormatCount(count))
		}
	}

	fmt.Printf("Stopping container %s...\n", containerName)
	if err := m.Docker.Stop(containerName); err != nil {
		return err
	}

	fmt.Printf("Container %s stopped\n", containerName)
	return nil
}

// Down stops and removes the container.
func (m *Manager) Down(workspaceFolder string, force bool) error {
	containerName := config.ContainerName(workspaceFolder)

	state := m.Docker.Inspect(containerName).State
	if state == docker.StateNotFound {
		fmt.Printf("No container found for %s\n", workspaceFolder)
		return nil
	}

	if state == docker.StateRunning {
		if !force {
			count := m.Session.Count(containerName)
			if count > 0 {
				return fmt.Errorf("container %s has %s; use --force to remove", containerName, session.FormatCount(count))
			}
		}
	}

	fmt.Printf("Removing container %s...\n", containerName)
	if err := m.Docker.Remove(containerName, true); err != nil {
		return err
	}

	m.Session.Clean(containerName)
	fmt.Printf("Container %s removed\n", containerName)
	return nil
}

// List returns all managed containers.
func (m *Manager) List() ([]types.ContainerInfo, error) {
	containers, err := m.Docker.ListManaged()
	if err != nil {
		return nil, err
	}

	for i := range containers {
		containers[i].Sessions = m.Session.Count(containers[i].Name)
	}

	return containers, nil
}

// Clean removes all stopped managed containers.
func (m *Manager) Clean(dryRun bool) ([]string, error) {
	containers, err := m.Docker.ListManaged()
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, c := range containers {
		if c.State == "stopped" {
			if dryRun {
				removed = append(removed, c.Name)
				continue
			}
			if err := m.Docker.Remove(c.Name, false); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", c.Name, err)
				continue
			}
			m.Session.Clean(c.Name)
			removed = append(removed, c.Name)
		}
	}

	return removed, nil
}

// linkAgentBinary ensures the agent's binary is on the system PATH by symlinking
// from the user-local install location (~/.local/bin) into /usr/local/bin.
func (m *Manager) linkAgentBinary(containerName string, profile *agent.Profile) {
	// Check common user-local install locations and symlink to /usr/local/bin
	cmd := fmt.Sprintf(
		`for dir in ~/.local/bin ~/bin ~/.claude/bin; do `+
			`if [ -x "$dir/%s" ]; then ln -sf "$dir/%s" /usr/local/bin/%s 2>/dev/null && break; fi; `+
			`done; true`,
		profile.Binary, profile.Binary, profile.Binary,
	)
	if err := m.Docker.ExecAs(containerName, []string{"sh", "-c", cmd}, docker.ExecOptions{User: "root"}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: could not link %s to PATH: %v\n", profile.Binary, err)
	}
}

// copyAgentConfig copies host agent configuration into the container.
// Files are copied (not mounted) so the container has its own writable copy
// with no link back to the host filesystem.
func (m *Manager) copyAgentConfig(containerName string, profile *agent.Profile, containerHome string) {
	home, _ := os.UserHomeDir()
	if home == "" {
		return
	}

	for _, mt := range profile.ConfigMounts {
		if !mt.Copy {
			continue
		}

		src := home + "/" + mt.HostPath
		if _, err := os.Stat(src); err != nil {
			continue // host path doesn't exist, skip
		}

		// Docker CopyToContainer extracts a tar into the destination directory.
		// For "~/.claude/settings.json" the tar contains "settings.json",
		// so the destination must be the parent: containerHome + "/.claude"
		dst := mt.ContainerPath
		if dst == "" {
			dst = containerHome + "/" + filepath.Dir(mt.HostPath)
		} else {
			dst = filepath.Dir(dst)
		}

		// Ensure destination directory exists with correct ownership
		mkdirCmd := fmt.Sprintf(`mkdir -p %s && chown -R 1000:1000 %s`, dst, dst)
		_ = m.Docker.ExecAs(containerName, []string{"sh", "-c", mkdirCmd}, docker.ExecOptions{User: "root"})

		if err := m.Docker.CopyInto(containerName, src, dst); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: could not copy %s into container: %v\n", mt.HostPath, err)
			continue
		}

		// Fix ownership so the container user can read/write
		target := containerHome + "/" + mt.HostPath
		if mt.ContainerPath != "" {
			target = mt.ContainerPath
		}
		chownCmd := fmt.Sprintf(`chown -R 1000:1000 %s 2>/dev/null; true`, target)
		_ = m.Docker.ExecAs(containerName, []string{"sh", "-c", chownCmd}, docker.ExecOptions{User: "root"})
	}
}

func (m *Manager) runLifecycleCommand(containerName string, cmd interface{}, name string) error {
	// Run lifecycle commands as the container user (not root), matching
	// devcontainer spec behavior. This ensures installs land in the
	// correct user home directory.
	opts := docker.ExecOptions{}

	switch v := cmd.(type) {
	case string:
		fmt.Printf("Running %s: %s\n", name, v)
		return m.Docker.ExecAs(containerName, []string{"sh", "-c", v}, opts)
	case []interface{}:
		args := make([]string, len(v))
		for i, a := range v {
			args[i] = fmt.Sprintf("%v", a)
		}
		fmt.Printf("Running %s: %v\n", name, args)
		return m.Docker.ExecAs(containerName, args, opts)
	default:
		return fmt.Errorf("unsupported command format for %s", name)
	}
}

// setupAgentPathMappings creates symlinks inside the container so agent trust/config
// that was established on the host (with host paths) also works for container paths.
//
// For example, Claude Code stores per-project trust in ~/.claude/projects/ keyed by
// the absolute workspace path. On the host this is /Users/graham/dev/myproject but
// in the container it's /workspaces/myproject. This method symlinks the container
// path entry to the host path entry so trust carries over.
func (m *Manager) setupAgentPathMappings(containerName string, profile *agent.Profile, hostWorkspace, containerWorkspace string) {
	switch profile.Name {
	case "claude":
		m.setupClaudePathMapping(containerName, hostWorkspace, containerWorkspace)
	}
}

func (m *Manager) setupClaudePathMapping(containerName, hostWorkspace, containerWorkspace string) {
	containerKey := claudeProjectKey(containerWorkspace)

	// Pre-create the project directory so Claude recognizes the workspace as trusted
	// and doesn't prompt the user on first run
	cmd := fmt.Sprintf(
		`home=$(eval echo ~) && `+
			`mkdir -p "$home/.claude/projects/%s"`,
		containerKey,
	)

	if err := m.Docker.ExecAs(containerName, []string{"sh", "-c", cmd}, docker.ExecOptions{}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: could not set up Claude project trust: %v\n", err)
	}
}

// claudeProjectKey converts an absolute path to the key Claude uses for
// ~/.claude/projects/ directory names: replace path separators with dashes.
func claudeProjectKey(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return strings.ReplaceAll(abs, string(filepath.Separator), "-")
}

// describeChanges compares stored container labels with current config to produce
// human-readable descriptions of what changed.
func describeChanges(
	labels map[string]string,
	devCfg *types.DevContainerConfig,
	custom *types.DevcCustomization,
	agentProfile *agent.Profile,
) []string {
	var changes []string

	if storedAgent := labels["devc.agent"]; storedAgent != "" {
		if custom.Agent != "" && custom.Agent != storedAgent {
			changes = append(changes, fmt.Sprintf("agent changed: %s → %s", storedAgent, custom.Agent))
		}
	} else if custom.Agent != "" {
		changes = append(changes, fmt.Sprintf("agent added: %s", custom.Agent))
	}

	if agentProfile != nil {
		changes = append(changes, fmt.Sprintf("agent %s profile may have updated features or install commands", agentProfile.Name))
	}

	// Generic fallback if we can't determine specifics
	if len(changes) == 0 {
		changes = append(changes, "devcontainer.json or devc configuration has changed")
	}

	return changes
}

// askYesNo reads a yes/no answer from stdin. Defaults to no.
func askYesNo() bool {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
