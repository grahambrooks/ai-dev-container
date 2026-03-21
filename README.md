# devc

AI-safe development containers with [devcontainer.json](https://containers.dev/) support.

`devc` creates isolated, sandboxed Docker containers for AI coding agents (Claude Code, Codex, Gemini CLI, Opencode) while providing a consistent development experience for both local and remote workflows.

## Features

- **Devcontainer.json compatible** — uses the standard [Dev Container spec](https://containers.dev/) with AI safety extensions via `customizations.devc`
- **Security profiles** — three presets (strict, moderate, permissive) controlling network access, capabilities, and resource limits
- **AI agent integration** — built-in profiles for Claude Code, Codex, Gemini CLI, and Opencode with config mounting and network allowlists
- **Session tracking** — per-container session counting prevents accidental stops while sessions are active
- **Persistent containers** — containers survive between sessions, resuming where you left off

## Install

```sh
go install github.com/grahambrooks/devc@latest
```

Or build from source:

```sh
git clone https://github.com/grahambrooks/devc.git
cd devc
make build
# binary at ./bin/devc
```

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- Go 1.22+ (for building from source)

## Quick start

```sh
# Initialize a project with AI safety defaults for Claude
devc init --agent claude

# Start the container and attach a shell
devc up

# Run a command inside the container
devc exec -- npm test

# Attach another session
devc attach

# Stop the container
devc stop
```

## Commands

| Command | Description |
|---|---|
| `devc up [path]` | Create and start a development container |
| `devc exec -- <cmd>` | Execute a command in a running container |
| `devc attach [path]` | Attach an interactive session |
| `devc stop [path]` | Stop a container (respects active sessions) |
| `devc down [path]` | Stop and remove a container |
| `devc build [path]` | Build or rebuild the container image |
| `devc list` | List all managed containers |
| `devc config [path]` | Display merged configuration |
| `devc clean` | Remove all stopped containers |
| `devc init [path]` | Generate a devcontainer.json with AI safety defaults |

### Global flags

```
--docker-path       Path to docker binary
--log-level         Log level: debug, info, warn, error (default: info)
--output-format     Output format: text, json (default: text)
```

## Configuration

### Project: `.devcontainer/devcontainer.json`

Standard devcontainer.json fields work as expected. AI safety settings go in `customizations.devc`:

```jsonc
{
  "name": "my-project",
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "features": {
    "ghcr.io/devcontainers/features/node:1": {}
  },
  "postCreateCommand": "npm install",
  "customizations": {
    "devc": {
      "agent": "claude",
      "securityProfile": "moderate",
      "network": {
        "mode": "restricted",
        "allowlist": ["api.anthropic.com", "registry.npmjs.org"]
      },
      "resources": {
        "cpus": "4",
        "memory": "8g",
        "pidsLimit": 256
      },
      "session": {
        "stopOnLastDetach": true
      }
    }
  }
}
```

### Global: `~/.devc/config.json`

User-level defaults that apply to all projects unless overridden at the project level.

### Security profiles

| Control | Strict | Moderate (default) | Permissive |
|---|---|---|---|
| Network | None | Domain allowlist | Host network |
| Capabilities | Drop ALL | Drop ALL + minimal | Docker defaults |
| Resources | 2 CPU, 4 GB | 4 CPU, 8 GB | Unlimited |
| User | Non-root | Non-root | Non-root |

## License

[MIT](LICENSE)
