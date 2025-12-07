# Styx - Claude Instructions

## Project Overview

Styx is a distributed system platform for Mac fleets using Apple Containers and HashiCorp Nomad.

**Target**: macOS 26+ with Apple Silicon

## Important Files

- `PLAN.md` - Living implementation plan with phases and checkboxes
- `TEST.md` - Testing requirements for each phase (run after completing a phase)
- `driver/` - Nomad task driver for Apple Containers
- `cmd/styx/` - Main CLI launcher (`styx init`, `styx join`, `styx stop`)
- `internal/` - Internal packages (config, launchd, network, proxy, vault)
- `example/` - Sample Nomad job specs for testing (alpine, nginx, ubuntu, nats, dragonfly)

## Directory Structure

```
styx/
├── cmd/styx/           # CLI commands
│   ├── main.go         # Entry point
│   ├── root.go         # Root command with global flags
│   ├── init.go         # styx init --server
│   ├── join.go         # styx join <server-ip>
│   ├── stop.go         # styx stop
│   └── version.go      # styx version
├── driver/             # Nomad task driver plugin
├── internal/
│   ├── config/         # Nomad/Vault HCL config generation
│   ├── launchd/        # macOS launchd plist management
│   ├── network/        # IP detection and Tailscale utilities
│   ├── proxy/          # TCP proxy for container port forwarding
│   └── vault/          # Vault initialization and setup helpers
├── example/            # Sample Nomad job specs
├── plugins/            # Built plugin binary
└── bin/                # Built CLI binary
```

## Rules

### Updating PLAN.md

- Mark tasks `[x]` when complete
- Mark tasks `[~]` when in progress
- Add new tasks discovered during implementation
- Keep phases in order but tasks within phases can be reordered
- Add notes under tasks if approach changed from original plan
- Update the Notes section at the bottom with discoveries

### Code Style

- Use `path/filepath` for file/directory paths (Go's equivalent of Pathlib)
- Minimize if/else and try/except - prefer single code paths
- Follow Go idioms (error returns, early returns)
- Keep functions small and focused

### No Backwards Compatibility

- Don't worry about backwards compatibility for existing installations
- Features should always be enabled (no optional flags for core functionality)
- Vault and security features are always on
- Breaking changes are acceptable - this is a development project

### No Sudo

- Styx must never require sudo to run
- All data stored in user directories (`~/.styx/`)
- Uses launchd user agents (not system daemons)

### Prefer Modern Approaches

When implementing features with HashiCorp tools (Nomad, Vault), always use the latest/modern approaches:

- **Vault + Nomad**: Use workload identities (JWT auth) instead of legacy token-based auth (Nomad 1.7+)
- **Vault Storage**: Use Raft (integrated storage) instead of Consul backend
- **Service Discovery**: Use Nomad native (`provider = "nomad"`) instead of Consul
- **Authentication**: Prefer short-lived tokens over long-lived static tokens
- **Service Registration**: Use `address_mode = "driver"` for container services to register Tailscale hostnames

### When Starting a Phase

1. Read PLAN.md to understand current state
2. Mark the first task as `[~]` in progress
3. Implement
4. Mark as `[x]` when done
5. Commit with message referencing the phase/task

### Commit Messages

Format: `phase X: description`

Examples:
- `phase 1: implement container start/stop lifecycle`
- `phase 1: add task driver skeleton`
- `phase 2: add styx init command`

### Package Management

The user uses **mise** to install and manage package versions. When a tool needs to be installed:
1. Check if it can be installed via mise first: `mise install <tool>`
2. Only fall back to Homebrew if mise doesn't support the tool

### Key Dependencies

- Nomad task driver SDK: `github.com/hashicorp/nomad/plugins/drivers`
- CLI framework: `github.com/spf13/cobra`
- Container runtime: `/usr/local/bin/container` (Apple's CLI)
- Nomad: `mise install nomad` or `brew install nomad`
- Vault: `mise install vault` or `brew install vault`
- Tailscale: https://tailscale.com/download (for cross-node networking and encryption)

### Running Commands

- **Nomad**: Run `nomad` commands directly without environment variables
- **Styx**: Run `./bin/styx` commands directly without sudo
- **Vault**: Run `vault` commands directly

### Testing

- **After completing a phase**: Run the corresponding tests in `TEST.md`
- **Quick smoke test**: See the "Quick Smoke Test" section in TEST.md
- Test task driver with `make dev` (builds plugin and starts Nomad in dev mode)
- Use `container` CLI directly to verify behavior before wrapping
- Example jobs in `example/` directory: alpine.nomad, nginx.nomad, ubuntu.nomad, nginx-vault.nomad

### When Completing a Phase

1. Run all tests for that phase from TEST.md
2. Mark all phase tasks as `[x]` in PLAN.md
3. Add checkmark to phase header in PLAN.md (e.g., `## Phase 1: Foundation ✓`)
4. Commit changes

### Architecture Notes

**Simplified Stack**: Styx uses Nomad + Vault + Tailscale (no Consul):
- **Orchestration**: Nomad schedules and runs containers
- **Secrets**: Vault with Raft storage provides secret management
- **Networking**: Tailscale handles cross-node communication and WireGuard encryption
- **Service Discovery**: Nomad native service discovery (`provider = "nomad"`)
- **DNS**: Tailscale MagicDNS for hostname resolution

**Port Convention**: Container ports are mapped to host ports at containerPort + 10000:
- Container port 80 → Host port 10080
- Container port 6379 → Host port 16379
