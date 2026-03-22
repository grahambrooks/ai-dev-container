# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
make build          # Build binary to bin/devc
make test           # go test ./...
make lint           # go vet ./...
make install        # go install .
go test ./internal/config   # Run tests for a single package
```

## Architecture

`devc` is an AI-safe dev container manager. It creates sandboxed Docker containers for AI coding agents (Claude, Codex,
Copilot, Gemini, Aider, Opencode) using the devcontainer.json spec, extended with AI safety controls under
`customizations.devc`.

### Configuration Cascade

```
~/.devc/config.json (global defaults)
  → .devcontainer/devcontainer.json (project overrides global)
    → CLI flags (override merged config)
```

Loading: `config.LoadGlobalConfig()` → `config.LoadDevcontainerConfig()` → `config.ExtractDevcCustomization()` →
`config.MergeCustomization()`. A SHA256 config hash is stored as a container label (`devc.config-hash`) for drift
detection on subsequent `devc up`.

### Key Layers

- **`cmd/`** — Cobra CLI commands. Each file is one command.
- **`internal/container/manager.go`** — Central orchestrator. `Up()` loads config, resolves agent profile, checks
  container state, creates/starts container, runs lifecycle commands. All container lifecycle flows through here.
- **`internal/docker/client.go`** — Wraps moby/moby Docker Engine API directly (no CLI shelling). `CreateAndStart()`
  assembles mounts, env vars, capabilities, network mode, and resource limits. `CopyInto()` copies host files into
  containers via tar archive.
- **`internal/agent/profiles.go`** — Declarative agent profiles: each defines `ConfigMounts` (what host config to
  copy/mount), `NetworkAllow` (domain allowlist), `InstallCmd` (binary install script), `EnvPassthrough` (API key env
  vars). `MountSpec.Copy=true` means copy into container (writable, no host link); `ReadOnly=true` means bind mount
  read-only.
- **`internal/agent/credentials.go`** — Extracts auth tokens from host. Claude: macOS Keychain (
  `Claude Code-credentials`) or env vars. GitHub (copilot/codex): macOS Keychain (`github.com` internet password) or
  `GH_TOKEN`/`GITHUB_TOKEN`.
- **`internal/security/profiles.go`** — Three presets (strict/moderate/permissive) controlling capabilities, network,
  resources. All profiles drop ALL caps then selectively add back `CHOWN`, `DAC_OVERRIDE`, `FOWNER` (needed for
  container setup).
- **`internal/config/hash.go`** — Config snapshot → JSON → SHA256. Includes agent mount specs so mount mode changes
  trigger rebuild.
- **`pkg/types/types.go`** — All shared types: `DevContainerConfig`, `DevcCustomization`, `SecurityProfile`, etc.

### Container Creation Flow

1. Pull/build image (with devcontainer features if any)
2. `CreateAndStart()` — set up mounts, env, security, network
3. `copyAgentConfig()` — `docker cp` host config files into container, `chown 1000:1000`
4. `setupAgentPathMappings()` — e.g., pre-create Claude trust directory
5. Lifecycle commands (`onCreateCommand` → `postCreateCommand` → `postStartCommand`) — run as container user, not root
6. `linkAgentBinary()` — symlink agent binary from `~/.local/bin` into `/usr/local/bin` (as root)

### Container Naming

`devc-{sanitized-basename}-{sha256(abs-path)[:8]}` — deterministic, allows same project name at different paths.

### Session Tracking

Lock files at `~/.devc/sessions/{container}.json` with PID arrays. Auto-prunes dead PIDs. Prevents accidental `stop`
while sessions are active.

## Key Conventions

- Container user is always `1000:1000` (vscode in devcontainer images). Lifecycle commands run as this user. Only
  `chown` and binary symlinking run as root.
- Agent install commands target `~/.local/bin` (user-writable), then `linkAgentBinary` symlinks to `/usr/local/bin`.
- Host auth (SSH keys, git config) is bind-mounted read-only via `CommonAuthMounts()`. Agent config that needs writes
  uses `Copy: true` (copied in, not mounted).
- The Docker client uses `github.com/moby/moby/client` v0.3.0 — check its types when working with Docker API calls (
  e.g., `VolumeCreateOptions`, `VolumeRemoveOptions` are client package types, not volume package types).
