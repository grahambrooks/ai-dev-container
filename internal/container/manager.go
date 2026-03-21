package container

import (
	"fmt"
	"os"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/internal/config"
	"github.com/grahambrooks/devc/internal/docker"
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
			fmt.Fprintf(os.Stderr, "warning: unknown agent %q, skipping agent configuration\n", merged.Agent)
		}
	}

	// Check existing container state
	state := m.Docker.Inspect(containerName)
	switch state {
	case docker.StateRunning:
		fmt.Printf("Container %s is already running\n", containerName)

	case docker.StateStopped, docker.StateCreated:
		fmt.Printf("Starting existing container %s...\n", containerName)
		if err := m.Docker.Start(containerName); err != nil {
			return fmt.Errorf("starting container: %w", err)
		}

	case docker.StateNotFound:
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
			built, err := m.Docker.BuildImageWithFeatures(devCfg.Image, devCfg.Features, containerName)
			if err != nil {
				return fmt.Errorf("building image with features: %w", err)
			}
			effectiveImage = built
		}

		// Swap the image for container creation
		origImage := devCfg.Image
		devCfg.Image = effectiveImage

		fmt.Printf("Creating container %s...\n", containerName)
		if err := m.Docker.CreateAndStart(containerName, devCfg, merged, opts.WorkspaceFolder, agentProfile); err != nil {
			// Restore original image ref before returning
			devCfg.Image = origImage
			return fmt.Errorf("creating container: %w", err)
		}
		devCfg.Image = origImage

		// Run lifecycle commands in order
		if devCfg.OnCreateCommand != nil {
			if err := m.runLifecycleCommand(containerName, devCfg.OnCreateCommand, "onCreateCommand"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: onCreateCommand failed: %v\n", err)
			}
		}
		if devCfg.PostCreateCommand != nil {
			if err := m.runLifecycleCommand(containerName, devCfg.PostCreateCommand, "postCreateCommand"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: postCreateCommand failed: %v\n", err)
			}
		}
		if devCfg.PostStartCommand != nil {
			if err := m.runLifecycleCommand(containerName, devCfg.PostStartCommand, "postStartCommand"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: postStartCommand failed: %v\n", err)
			}
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

// Exec runs a command in the container for the given workspace.
func (m *Manager) Exec(workspaceFolder string, command []string) error {
	containerName := config.ContainerName(workspaceFolder)

	state := m.Docker.Inspect(containerName)
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

	state := m.Docker.Inspect(containerName)
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

	state := m.Docker.Inspect(containerName)
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

	state := m.Docker.Inspect(containerName)
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

	// Attach session counts
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
				fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", c.Name, err)
				continue
			}
			m.Session.Clean(c.Name)
			removed = append(removed, c.Name)
		}
	}

	return removed, nil
}

func (m *Manager) runLifecycleCommand(containerName string, cmd interface{}, name string) error {
	opts := docker.ExecOptions{User: "root"}

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
