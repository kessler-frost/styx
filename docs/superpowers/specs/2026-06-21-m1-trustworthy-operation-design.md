# Styx M1 — Trustworthy Single-Cluster Operation (design)

**Date:** 2026-06-21
**Status:** Approved design, pending implementation plan
**Milestone:** M1 of a two-milestone roadmap. M2 (container-machine workloads) is a separate spec built on this.

## Goal

styx becomes reliable to **start, operate, and stop** for real single-cluster use: it fails safe on interruption, can't exhaust the disk, recovers from transient blips, has a CI gate against regressions — and sheds its heaviest, most fragile dependency (**Vault**), replacing secrets with **Nomad native variables**. Everything in M1 is unit/mock-testable on macOS; the real multi-Mac cluster e2e stays a documented gated smoke test.

## Context

styx is feature-complete (Nomad + Apple Containers + Tailscale + platform services, TUI/CLI). Wave 1 added its first 37 unit tests and fixed 6 bugs (incl. the always-`0` container exit code and silent error-swallowing). M1 turns "feature-complete" into "trustworthy to run."

## Workstream A — Remove Vault, adopt Nomad native variables

Vault was the heaviest external dependency and operationally fragile (seal/unseal lifecycle, a placeholder `key-shares=1` setup). It is removed entirely; secrets move to Nomad's built-in secure variables.

**Remove:**
- Packages/files: `internal/vault/`, `internal/config/vault_templates.go`, `example/nginx-vault.nomad`.
- Deps: `github.com/hashicorp/vault/api` and the `hcl-vault` indirect from `go.mod`/`go.sum`.
- Wiring (compiler-driven: remove → fix dangling refs by removing → re-verify): `cmd/styx/init.go` (initialize/unseal/`SetupNomadIntegration`, the auto-unseal launchd script fragment, `ensureVaultUnsealed`), `cmd/styx/status.go` (sealed check), `internal/setup/{install,setup,check}.go` (Vault prerequisite + install), `internal/api/{api,cluster,types}.go` (Vault in cluster status), `internal/tui/views/cluster.go` (Vault row), `internal/services/jobs.go`, `internal/constants/ports.go` (Vault port), `cmd/styx/{chaos,uninstall,root}.go`.
- Tests referencing Vault, updated to match: `internal/config/generator_test.go`, `cmd/styx/status_test.go`, `internal/constants/ports_test.go`, `internal/api/types_test.go`.
- Docs: README, `.claude/PLAN.md`, `.claude/TEST.md`, `.claude/CLAUDE.md`, `.claude/agents/chaos-tester.md` — drop Vault, document the new secrets model.

**Replace with Nomad native variables:**
- Enable Nomad secure variables + workload identity in the generated server/client config so jobs can read their own variables via `{{ with nomadVar "…" }}` templating.
- Provide a minimal styx affordance — a thin `styx secret put/get` helper over `nomad var`, or (if simpler) documented `nomad var` usage + a job-template pattern. Keep the styx-side surface small.
- Acceptance: a workload can consume a secret sourced from a Nomad variable, with no Vault process anywhere; `go build ./... && go vet ./... && go test ./...` green after removal.

## Workstream B — Safety & lifecycle

- **Init rollback** — `styx init` becomes transactional: each setup step (Nomad agent, configs, launchd, networking) registers an undo; a mid-sequence failure runs the undo stack back to a clean state rather than leaving a half-cluster. (Simpler now that Vault's multi-step setup is gone.)
- **Graceful shutdown** — SIGINT/SIGTERM handlers in the server/daemon paths: drain, stop jobs/services, flush state before exit.
- **Verified, idempotent teardown** — `stop`/`uninstall` fully and re-runnably tear down, every error surfaced (no silent `_ =` discards; Wave 1 began this).

## Workstream C — Operational robustness

- **Log rotation** — size+age-bounded rotation for `~/.styx/logs/*` (nomad.log etc.) so logs can't exhaust the disk.
- **Health-check retries/backoff** — replace fixed one-shot 2s timeouts with retry + exponential backoff + a clear terminal error (eliminates false "already running" and transient-blip failures).
- **Subnet-collision detection** — before claiming the container subnet (hardcoded `192.168.200.0/24`), check host routes/interfaces; pick/validate a free subnet, fail clearly if none.

## Workstream D — Confidence (tests + CI)

- **Mock-client integration tests** — put the Nomad client behind an interface (or `httptest` servers) so the init sequence, service deploy/wait loops, and status aggregation are testable without a live cluster — extends the Wave 1 suite toward the real boundary.
- **CI gate** — GitHub Actions on push/PR (macOS runner): `go build ./...`, `go vet ./...`, `gofmt -l` check, `go test ./...`, golangci-lint. Live Nomad/Tailscale/Apple-Container bits stay gated; build + unit + mock run.

## Error-handling principles

Every teardown/rollback path surfaces its errors (no silent `_ =`). Operations that can partially fail are made idempotent so re-running converges to a clean state.

## Testing strategy

All of M1 is unit/mock-testable on macOS (the point of Workstream D). The genuine multi-Mac cluster e2e (real Nomad/Tailscale/Apple Containers) and the in-cluster Nomad-variables secret read remain documented, gated smoke tests.

## Out of scope (M1)

Container-machine workloads (M2), multi-cluster federation, observability-stack changes. No Vault replacement beyond Nomad native variables.
