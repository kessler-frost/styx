# Styx - Claude Instructions

## Project Overview

Styx is a distributed system platform for Mac fleets using Apple Containers and HashiCorp Nomad.

**Target**: macOS 26+ with Apple Silicon

## Important Files

- `PLAN.md` - Living implementation plan with phases and checkboxes
- `driver/` - Nomad task driver for Apple Containers
- `cmd/styx/` - Main CLI launcher

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

### Key Dependencies

- Nomad task driver SDK: `github.com/hashicorp/nomad/plugins/drivers`
- CLI framework: `github.com/spf13/cobra`
- Container runtime: `/usr/local/bin/container` (Apple's CLI)

### Testing

- Test task driver with `nomad agent -dev`
- Use `container` CLI directly to verify behavior before wrapping
