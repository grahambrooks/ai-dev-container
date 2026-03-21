package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/internal/config"
	"github.com/grahambrooks/devc/internal/security"
	"github.com/grahambrooks/devc/pkg/types"
)

// Client wraps Docker CLI operations.
type Client struct {
	DockerPath string
}

// NewClient creates a Docker client, locating the docker binary.
func NewClient(dockerPath string) (*Client, error) {
	if dockerPath == "" {
		path, err := exec.LookPath("docker")
		if err != nil {
			return nil, fmt.Errorf("docker not found in PATH: %w", err)
		}
		dockerPath = path
	}
	return &Client{DockerPath: dockerPath}, nil
}

// ContainerState describes the state of a Docker container.
type ContainerState string

const (
	StateRunning  ContainerState = "running"
	StateStopped  ContainerState = "stopped"
	StateNotFound ContainerState = "not_found"
	StateCreated  ContainerState = "created"
)

// Inspect returns the state of a container by name.
func (c *Client) Inspect(name string) ContainerState {
	out, err := c.run("inspect", "--format", "{{.State.Status}}", name)
	if err != nil {
		return StateNotFound
	}
	status := strings.TrimSpace(out)
	switch status {
	case "running":
		return StateRunning
	case "exited", "dead":
		return StateStopped
	case "created":
		return StateCreated
	default:
		return StateStopped
	}
}

// ImageExists checks whether a Docker image exists locally.
func (c *Client) ImageExists(image string) bool {
	_, err := c.run("image", "inspect", image)
	return err == nil
}

// Pull pulls a Docker image.
func (c *Client) Pull(image string) error {
	return c.runInteractive("pull", image)
}

// CreateAndStart creates and starts a container with the given configuration.
func (c *Client) CreateAndStart(
	containerName string,
	devCfg *types.DevContainerConfig,
	custom *types.DevcCustomization,
	workspaceFolder string,
	agentProfile *agent.Profile,
) error {
	args := []string{"run", "-d",
		"--name", containerName,
		"--label", "devc.managed=true",
		"--label", "devc.workspace=" + workspaceFolder,
	}

	// Workspace mount
	wsTarget := config.WorkspaceInContainer(devCfg, workspaceFolder)
	mountMode := "rw"
	if custom.Filesystem != nil && custom.Filesystem.ProjectMountMode != "" {
		mountMode = custom.Filesystem.ProjectMountMode
	}
	args = append(args, "-v", workspaceFolder+":"+wsTarget+":"+mountMode)
	args = append(args, "-w", wsTarget)

	// Security profile
	profile := security.GetProfile(custom.SecurityProfile)
	args = append(args, applySecurityArgs(profile, custom)...)

	// Environment variables
	for k, v := range devCfg.ContainerEnv {
		args = append(args, "-e", k+"="+v)
	}

	// Agent-specific mounts
	if agentProfile != nil {
		home, _ := os.UserHomeDir()
		if home != "" {
			args = append(args, agentProfile.ConfigMounts(home)...)
		}
		args = append(args, "--label", "devc.agent="+agentProfile.Name)
	}

	// Override command to keep container running
	if devCfg.OverrideCommand == nil || *devCfg.OverrideCommand {
		args = append(args, devCfg.Image, "sleep", "infinity")
	} else {
		args = append(args, devCfg.Image)
	}

	return c.runInteractive(args...)
}

// Start starts a stopped container.
func (c *Client) Start(name string) error {
	return c.runInteractive("start", name)
}

// Stop stops a running container.
func (c *Client) Stop(name string) error {
	return c.runInteractive("stop", name)
}

// Remove removes a container.
func (c *Client) Remove(name string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)
	return c.runInteractive(args...)
}

// Exec runs a command inside a running container.
func (c *Client) Exec(name string, command []string, interactive bool) error {
	args := []string{"exec"}
	if interactive {
		args = append(args, "-it")
	}
	args = append(args, name)
	args = append(args, command...)

	cmd := exec.Command(c.DockerPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ListManaged returns all containers with the devc.managed label.
func (c *Client) ListManaged() ([]types.ContainerInfo, error) {
	out, err := c.run("ps", "-a",
		"--filter", "label=devc.managed=true",
		"--format", "{{.Names}}\t{{.ID}}\t{{.Status}}\t{{.Image}}\t{{.Label \"devc.workspace\"}}\t{{.Label \"devc.agent\"}}",
	)
	if err != nil {
		return nil, err
	}

	var containers []types.ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 6)
		if len(parts) < 5 {
			continue
		}

		state := "stopped"
		if strings.HasPrefix(parts[2], "Up") {
			state = "running"
		}

		info := types.ContainerInfo{
			Name:            parts[0],
			ContainerID:     parts[1],
			State:           state,
			Image:           parts[3],
			WorkspaceFolder: parts[4],
		}
		if len(parts) > 5 {
			info.Agent = parts[5]
		}
		containers = append(containers, info)
	}
	return containers, nil
}

func applySecurityArgs(profile *types.SecurityProfile, custom *types.DevcCustomization) []string {
	var args []string

	// Capabilities
	if profile.DropAllCaps {
		args = append(args, "--cap-drop=ALL")
		for _, cap := range profile.AddCaps {
			args = append(args, "--cap-add="+cap)
		}
	}

	// Resource limits
	res := profile.Resources
	if custom.Resources != nil {
		if custom.Resources.CPUs != "" {
			res.CPUs = custom.Resources.CPUs
		}
		if custom.Resources.Memory != "" {
			res.Memory = custom.Resources.Memory
		}
		if custom.Resources.PidsLimit > 0 {
			res.PidsLimit = custom.Resources.PidsLimit
		}
	}

	if res.CPUs != "" {
		args = append(args, "--cpus="+res.CPUs)
	}
	if res.Memory != "" {
		args = append(args, "--memory="+res.Memory)
	}
	if res.PidsLimit > 0 {
		args = append(args, fmt.Sprintf("--pids-limit=%d", res.PidsLimit))
	}

	// Network
	netMode := profile.Network.Mode
	if custom.Network != nil && custom.Network.Mode != "" {
		netMode = custom.Network.Mode
	}
	switch netMode {
	case "none":
		args = append(args, "--network=none")
	case "host":
		args = append(args, "--network=host")
		// "restricted" uses the default bridge; network policies applied separately
	}

	// User
	if profile.RunAsUser != "" {
		args = append(args, "--user="+profile.RunAsUser)
	}

	// Security options
	args = append(args, "--security-opt=no-new-privileges")

	return args
}

func (c *Client) run(args ...string) (string, error) {
	cmd := exec.Command(c.DockerPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func (c *Client) runInteractive(args ...string) error {
	cmd := exec.Command(c.DockerPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
