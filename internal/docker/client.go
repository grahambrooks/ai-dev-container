package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	dockerclient "github.com/moby/moby/client"
	"golang.org/x/term"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/internal/config"
	"github.com/grahambrooks/devc/internal/security"
	"github.com/grahambrooks/devc/pkg/types"
)

// Client wraps the Docker Engine API.
type Client struct {
	api *dockerclient.Client
}

// NewClient creates a Docker API client from the environment.
// It reads DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH, and DOCKER_TLS_VERIFY.
func NewClient() (*Client, error) {
	api, err := dockerclient.New(dockerclient.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	// Verify connectivity with a ping
	ctx := context.Background()
	if _, err := api.Ping(ctx, dockerclient.PingOptions{}); err != nil {
		return nil, fmt.Errorf(`cannot connect to a container runtime.

devc requires a running container runtime with a Docker-compatible API.

Supported runtimes:
  - Docker Desktop    https://www.docker.com/products/docker-desktop/
  - Colima            https://github.com/abiosoft/colima
  - Rancher Desktop   https://rancherdesktop.io/ (moby mode)
  - OrbStack          https://orbstack.dev/
  - Podman            https://podman.io/

If the runtime is running but devc cannot find it, set DOCKER_HOST:
  export DOCKER_HOST="unix:///path/to/docker.sock"

Underlying error: %w`, err)
	}

	return &Client{api: api}, nil
}

// Close releases the Docker client resources.
func (c *Client) Close() error {
	return c.api.Close()
}

// ContainerState describes the state of a Docker container.
type ContainerState string

const (
	StateRunning  ContainerState = "running"
	StateStopped  ContainerState = "stopped"
	StateNotFound ContainerState = "not_found"
	StateCreated  ContainerState = "created"
)

// ContainerInspectResult holds the state and labels of an inspected container.
type ContainerInspectResult struct {
	State  ContainerState
	Labels map[string]string
}

// Inspect returns the state and metadata of a container by name.
func (c *Client) Inspect(name string) ContainerInspectResult {
	ctx := context.Background()
	result, err := c.api.ContainerInspect(ctx, name, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return ContainerInspectResult{State: StateNotFound}
	}

	var state ContainerState
	switch result.Container.State.Status {
	case "running":
		state = StateRunning
	case "exited", "dead":
		state = StateStopped
	case "created":
		state = StateCreated
	default:
		state = StateStopped
	}

	return ContainerInspectResult{
		State:  state,
		Labels: result.Container.Config.Labels,
	}
}

// ImageExists checks whether a Docker image exists locally.
func (c *Client) ImageExists(image string) bool {
	ctx := context.Background()
	_, err := c.api.ImageInspect(ctx, image)
	return err == nil
}

// Pull pulls a Docker image.
func (c *Client) Pull(image string) error {
	ctx := context.Background()
	resp, err := c.api.ImagePull(ctx, image, dockerclient.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer resp.Close()
	// Drain the reader to complete the pull; stream to stdout for progress
	_, err = io.Copy(os.Stdout, resp)
	return err
}

// CreateAndStart creates and starts a container with the given configuration.
func (c *Client) CreateAndStart(
	containerName string,
	devCfg *types.DevContainerConfig,
	custom *types.DevcCustomization,
	workspaceFolder string,
	agentProfile *agent.Profile,
	configHash string,
) error {
	ctx := context.Background()

	wsTarget := config.WorkspaceInContainer(devCfg, workspaceFolder)

	// Build container config
	cmd := []string{"sleep", "infinity"}
	if devCfg.OverrideCommand != nil && !*devCfg.OverrideCommand {
		cmd = nil
	}

	env := make([]string, 0, len(devCfg.ContainerEnv))
	for k, v := range devCfg.ContainerEnv {
		env = append(env, k+"="+v)
	}

	// Forward host env vars for auth (API keys, tokens)
	// Collect from both agent profile and devc customization, deduplicating
	passthroughSet := make(map[string]bool)
	if agentProfile != nil {
		for _, envName := range agentProfile.EnvPassthrough {
			passthroughSet[envName] = true
		}
		for k, v := range agentProfile.EnvVars {
			env = append(env, k+"="+v)
		}
	}
	if custom.EnvPassthrough != nil {
		for _, envName := range custom.EnvPassthrough {
			passthroughSet[envName] = true
		}
	}
	for envName := range passthroughSet {
		if val, ok := os.LookupEnv(envName); ok {
			env = append(env, envName+"="+val)
		}
	}

	// Resolve agent credentials from host (Keychain, credential files, etc.)
	if agentProfile != nil {
		creds := agent.ResolveCredentials(agentProfile)
		env = append(env, creds.Env...)
	}

	labels := map[string]string{
		"devc.managed":     "true",
		"devc.workspace":   workspaceFolder,
		"devc.config-hash": configHash,
	}
	if agentProfile != nil {
		labels["devc.agent"] = agentProfile.Name
	}

	// Resolve security profile and effective user early (needed for mount targets)
	profile := security.GetProfile(custom.SecurityProfile)
	effectiveUser := profile.RunAsUser

	containerCfg := &container.Config{
		Image:      devCfg.Image,
		Cmd:        cmd,
		Env:        env,
		Labels:     labels,
		WorkingDir: wsTarget,
		User:       effectiveUser,
	}

	// Build host config
	mountMode := "rw"
	if custom.Filesystem != nil && custom.Filesystem.ProjectMountMode != "" {
		mountMode = custom.Filesystem.ProjectMountMode
	}
	readOnly := mountMode == "ro"

	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   workspaceFolder,
			Target:   wsTarget,
			ReadOnly: readOnly,
		},
	}

	// Agent config and auth mounts — target the container user's actual home
	home, _ := os.UserHomeDir()
	containerHome := containerHomeDir(ctx, c.api, devCfg.Image, effectiveUser)

	if agentProfile != nil && home != "" {
		for _, m := range agentProfile.ConfigMounts {
			src := home + "/" + m.HostPath
			// Skip mounts where the source doesn't exist on the host
			if _, statErr := os.Stat(src); statErr != nil {
				continue
			}
			dst := m.ContainerPath
			if dst == "" {
				dst = containerHome + "/" + m.HostPath
			}

			if m.Seed {
				// Seed mount: create a named volume at the target path,
				// bind-mount host dir read-only to a temp seed location
				volName := agent.SeedVolumeName(containerName, m.HostPath)
				if err := c.CreateVolume(volName, map[string]string{
					"devc.managed":   "true",
					"devc.container": containerName,
					"devc.seed-path": m.HostPath,
				}); err != nil {
					return fmt.Errorf("creating seed volume %s: %w", volName, err)
				}
				// Volume mount at target (writable)
				mounts = append(mounts, mount.Mount{
					Type:   mount.TypeVolume,
					Source: volName,
					Target: dst,
				})
				// Bind mount host content to temp seed location (read-only)
				mounts = append(mounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   src,
					Target:   agent.SeedPath(m.HostPath),
					ReadOnly: true,
				})
			} else {
				mounts = append(mounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   src,
					Target:   dst,
					ReadOnly: m.ReadOnly,
				})
			}
		}
	}

	// Common auth mounts (SSH keys, git config) — always read-only
	if home != "" {
		for _, m := range agent.CommonAuthMounts() {
			src := home + "/" + m.HostPath
			if _, statErr := os.Stat(src); statErr != nil {
				continue
			}
			dst := containerHome + "/" + m.HostPath
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   src,
				Target:   dst,
				ReadOnly: true,
			})
		}
	}

	// SSH agent socket forwarding
	if hostSock, containerSock := agent.SSHAuthSockMount(); hostSock != "" {
		if _, statErr := os.Stat(hostSock); statErr == nil {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   hostSock,
				Target:   containerSock,
				ReadOnly: true,
			})
			env = append(env, "SSH_AUTH_SOCK="+containerSock)
		}
	}

	hostCfg := &container.HostConfig{
		Mounts:      mounts,
		SecurityOpt: []string{"no-new-privileges"},
	}

	// Capabilities
	if profile.DropAllCaps {
		hostCfg.CapDrop = []string{"ALL"}
		hostCfg.CapAdd = profile.AddCaps
	}

	// Resources
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
		cpus, err := strconv.ParseFloat(res.CPUs, 64)
		if err == nil {
			hostCfg.Resources.NanoCPUs = int64(cpus * 1e9)
		}
	}
	if res.Memory != "" {
		hostCfg.Resources.Memory = parseMemoryString(res.Memory)
	}
	if res.PidsLimit > 0 {
		hostCfg.Resources.PidsLimit = &res.PidsLimit
	}

	// Network
	netMode := profile.Network.Mode
	if custom.Network != nil && custom.Network.Mode != "" {
		netMode = custom.Network.Mode
	}
	switch netMode {
	case "none":
		hostCfg.NetworkMode = "none"
	case "host":
		hostCfg.NetworkMode = "host"
	default:
		hostCfg.NetworkMode = "bridge"
	}

	// Create container
	createResult, err := c.api.ContainerCreate(ctx, dockerclient.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       containerName,
	})
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	fmt.Println(createResult.ID)

	// Start container
	_, err = c.api.ContainerStart(ctx, createResult.ID, dockerclient.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	return nil
}

// Start starts a stopped container.
func (c *Client) Start(name string) error {
	ctx := context.Background()
	_, err := c.api.ContainerStart(ctx, name, dockerclient.ContainerStartOptions{})
	return err
}

// Stop stops a running container.
func (c *Client) Stop(name string) error {
	ctx := context.Background()
	timeout := 10
	_, err := c.api.ContainerStop(ctx, name, dockerclient.ContainerStopOptions{
		Timeout: &timeout,
	})
	return err
}

// Remove removes a container.
func (c *Client) Remove(name string, force bool) error {
	ctx := context.Background()
	_, err := c.api.ContainerRemove(ctx, name, dockerclient.ContainerRemoveOptions{
		Force: force,
	})
	return err
}

// ExecOptions configures a docker exec call.
type ExecOptions struct {
	Interactive bool
	User        string
}

// Exec runs a command inside a running container.
func (c *Client) Exec(name string, command []string, interactive bool) error {
	return c.ExecAs(name, command, ExecOptions{Interactive: interactive})
}

// ExecAs runs a command inside a running container with additional options.
func (c *Client) ExecAs(name string, command []string, opts ExecOptions) error {
	ctx := context.Background()

	isTTY := opts.Interactive && term.IsTerminal(int(os.Stdin.Fd()))

	createOpts := dockerclient.ExecCreateOptions{
		Cmd:          command,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  opts.Interactive,
		TTY:          isTTY,
	}
	if opts.User != "" {
		createOpts.User = opts.User
	}

	execResult, err := c.api.ExecCreate(ctx, name, createOpts)
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	attachResult, err := c.api.ExecAttach(ctx, execResult.ID, dockerclient.ExecAttachOptions{
		TTY: isTTY,
	})
	if err != nil {
		return fmt.Errorf("attaching exec: %w", err)
	}
	defer attachResult.Close()

	if isTTY {
		// Set terminal to raw mode for interactive sessions
		if oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd())); rawErr == nil {
			defer term.Restore(int(os.Stdin.Fd()), oldState)
		}

		// Bidirectional copy
		go func() {
			_, _ = io.Copy(attachResult.Conn, os.Stdin)
		}()
		_, _ = io.Copy(os.Stdout, attachResult.Reader)
	} else if opts.Interactive {
		// Non-TTY but interactive (stdin piped)
		go func() {
			_, _ = io.Copy(attachResult.Conn, os.Stdin)
			attachResult.CloseWrite()
		}()
		_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, attachResult.Reader)
	} else {
		// Non-interactive: demux stdout/stderr
		_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, attachResult.Reader)
	}

	// Check exit code
	inspectResult, err := c.api.ExecInspect(ctx, execResult.ID, dockerclient.ExecInspectOptions{})
	if err != nil {
		return nil // Can't determine exit code, assume success
	}
	if inspectResult.ExitCode != 0 {
		return fmt.Errorf("exit status %d", inspectResult.ExitCode)
	}

	return nil
}

// ListManaged returns all containers with the devc.managed label.
func (c *Client) ListManaged() ([]types.ContainerInfo, error) {
	ctx := context.Background()

	f := make(dockerclient.Filters)
	f.Add("label", "devc.managed=true")

	result, err := c.api.ContainerList(ctx, dockerclient.ContainerListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, err
	}

	var containers []types.ContainerInfo
	for _, ctr := range result.Items {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}

		state := "stopped"
		if ctr.State == "running" {
			state = "running"
		}

		info := types.ContainerInfo{
			Name:            name,
			ContainerID:     ctr.ID[:12],
			State:           state,
			Image:           ctr.Image,
			WorkspaceFolder: ctr.Labels["devc.workspace"],
			Agent:           ctr.Labels["devc.agent"],
		}
		containers = append(containers, info)
	}
	return containers, nil
}

// BuildImageWithFeatures builds a custom image with devcontainer features installed.
func (c *Client) BuildImageWithFeatures(
	baseImage string,
	features map[string]interface{},
	containerName string,
) (string, error) {
	if len(features) == 0 {
		return baseImage, nil
	}

	tag := buildTag(baseImage, features, containerName)

	if c.ImageExists(tag) {
		return tag, nil
	}

	dockerfile := generateDockerfile(baseImage, features)

	// Create tar archive as build context
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	dfBytes := []byte(dockerfile)
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dfBytes)),
		Mode: 0644,
	}); err != nil {
		return "", err
	}
	if _, err := tw.Write(dfBytes); err != nil {
		return "", err
	}
	if err := tw.Close(); err != nil {
		return "", err
	}

	ctx := context.Background()
	fmt.Println("Building image with features...")

	resp, err := c.api.ImageBuild(ctx, &buf, dockerclient.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
		Remove:     true,
	})
	if err != nil {
		return "", fmt.Errorf("building image: %w", err)
	}
	defer resp.Body.Close()

	// Drain build output to stdout
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return "", fmt.Errorf("reading build output: %w", err)
	}

	return tag, nil
}

// CreateVolume creates a named Docker volume with the given labels. Idempotent.
func (c *Client) CreateVolume(name string, labels map[string]string) error {
	ctx := context.Background()
	_, err := c.api.VolumeCreate(ctx, dockerclient.VolumeCreateOptions{
		Name:   name,
		Labels: labels,
	})
	return err
}

// RemoveContainerVolumes removes all seed volumes associated with a container.
func (c *Client) RemoveContainerVolumes(containerName string) error {
	ctx := context.Background()

	f := make(dockerclient.Filters)
	f.Add("label", "devc.container="+containerName)

	result, err := c.api.VolumeList(ctx, dockerclient.VolumeListOptions{Filters: f})
	if err != nil {
		return fmt.Errorf("listing volumes: %w", err)
	}

	for _, v := range result.Items {
		if _, removeErr := c.api.VolumeRemove(ctx, v.Name, dockerclient.VolumeRemoveOptions{Force: true}); removeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: could not remove volume %s: %v\n", v.Name, removeErr)
		}
	}
	return nil
}

func parseMemoryString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	multiplier := int64(1)
	switch {
	case strings.HasSuffix(s, "g") || strings.HasSuffix(s, "G"):
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "m") || strings.HasSuffix(s, "M"):
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "k") || strings.HasSuffix(s, "K"):
		multiplier = 1024
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return val * multiplier
}

// containerHomeDir determines the home directory for the effective user inside
// the container image. It inspects the image to find the configured user,
// then maps common devcontainer users to their home directories.
func containerHomeDir(ctx context.Context, api *dockerclient.Client, imageName string, overrideUser string) string {
	user := overrideUser

	// If no user override, check image config
	if user == "" {
		result, err := api.ImageInspect(ctx, imageName)
		if err == nil && result.Config != nil {
			user = result.Config.User
		}
	}

	// Extract username from uid:gid format
	if idx := strings.Index(user, ":"); idx != -1 {
		user = user[:idx]
	}

	switch user {
	case "", "root", "0":
		return "/root"
	case "vscode", "1000":
		return "/home/vscode"
	case "node":
		return "/home/node"
	default:
		return "/home/" + user
	}
}
