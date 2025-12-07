# Styx Implementation Plan

> A distributed system platform for Mac fleets using Apple Containers + Nomad

## Status Legend
- [ ] Not started
- [~] In progress
- [x] Complete

---

## Phase 1: Foundation ✓
**Goal**: Run Apple Containers via Nomad locally

- [x] Create Nomad task driver skeleton
- [x] Implement container CLI wrapper
- [x] Basic lifecycle: start, stop, destroy
- [x] Parse container inspect output
- [x] Test with `nomad agent -dev`

**Deliverable**: `nomad job run` launches an Apple Container (COMPLETE)

---

## Phase 2: Cluster Basics ✓
**Goal**: Multiple Macs form a cluster

- [x] Styx launcher CLI skeleton (cobra)
- [x] Download/manage Nomad binary (prerequisite: `brew install nomad`)
- [x] Generate Nomad server/client configs
- [x] launchd service integration
- [x] `styx init` / `styx join` commands
- [x] Auto-discover servers via Tailscale (`styx` with no args)
  - Run `tailscale status` to get connected devices and IPs
  - Probe each IP for running Nomad server (port 4646)
  - Auto-join first discovered server (or prompt if multiple found)
  - Prompt to start server if none found

**Deliverable**: 2+ Macs running as Nomad cluster (COMPLETE)

---

## Phase 3: Service Discovery ✓
**Goal**: Services find each other

- [x] Nomad native service discovery (`provider = "nomad"`)
- [x] Services registered with Tailscale DNS names
- [x] Health checks via Nomad
- [x] Task driver returns DriverNetwork with Tailscale hostname
- [x] Service registration with `address_mode = "driver"`

**Note**: Using Nomad native service discovery instead of Consul.
Services are accessed via Tailscale MagicDNS (hostname.ts.net:port).

**Deliverable**: Services discoverable via `nomad service list` (COMPLETE)

---

## Phase 4: Networking ✓
**Goal**: Containers communicate across Macs via Tailscale

- [x] Tailscale detection utility (`internal/network/tailscale.go`)
- [x] TCP proxy for port forwarding (`internal/proxy/tcp.go`)
- [x] Task driver starts proxies and returns Tailscale hostname
- [x] Services registered with Tailscale MagicDNS names
- [x] Health checks now work via host port proxy
- [x] init/join commands show Tailscale status

**Deliverable**: Container on Mac A talks to container on Mac B (COMPLETE)

---

## Phase 5: Security ✓
**Goal**: Secure communication and secrets

- [x] Vault integration for secrets (server mode only, Raft storage)
- [x] Vault-Nomad integration via workload identities (JWT auth)
- [x] Tailscale WireGuard for transport encryption

**Note**: Tailscale provides encrypted transport between all nodes via WireGuard.
No additional TLS or gossip encryption needed when all nodes are on the same Tailnet.

**Deliverable**: Vault for secrets, Tailscale for transport encryption (COMPLETE)

---

## Phase 6: Distributed Primitives ✓
**Goal**: Cache, queue, storage available to services

- [x] Deploy NATS as Nomad job (message queue)
- [x] Deploy Dragonfly as Nomad job (Redis-compatible cache, replaced Olric which lacks ARM64)
- [ ] Deploy S3-compatible storage (deferred - see Notes)
- [x] Example Go client for Dragonfly (`example/test-dragonfly/`)

**Deliverable**: Services can use redis:// nats:// endpoints (COMPLETE - s3:// deferred)

---

## Phase 7: Ingress & Load Balancing
**Goal**: External traffic reaches services

- [ ] Deploy Traefik as Nomad job
- [ ] Nomad provider integration (reads from Nomad service catalog)
- [ ] TLS termination
- [ ] Load balancing across replicas

**Note**: Traefik will use Nomad provider directly (no Consul needed).

**Deliverable**: External requests routed to correct service

---

## Phase 8: Observability
**Goal**: See what's happening

- [ ] Centralized logging (Loki or similar)
- [ ] Metrics collection (Prometheus/VictoriaMetrics)
- [ ] Basic dashboards (Grafana)
- [ ] Optional: Distributed tracing (Jaeger)

**Deliverable**: Logs, metrics, traces in one place

---

## Phase 9: Resilience
**Goal**: System handles failures gracefully

- [ ] Backup strategy for stateful services
- [ ] Disaster recovery procedures
- [ ] Chaos testing (kill nodes, partitions)

**Deliverable**: Documented recovery procedures, tested

---

## Phase 10: SSH TUI
**Goal**: Visual cluster management

- [ ] SSH server (charmbracelet/wish)
- [ ] TUI views (bubbletea)
- [ ] View nodes, jobs, logs, metrics

**Deliverable**: `ssh styx.local` gives management interface

---

## Component Reference

| Component | Solution | Phase |
|-----------|----------|-------|
| Orchestration | Nomad | 1 |
| Container Runtime | Apple `container` CLI | 1 |
| Service Discovery | Nomad Native | 3 |
| DNS | Tailscale MagicDNS | 4 |
| Networking | Tailscale | 4 |
| Transport Encryption | Tailscale WireGuard | 5 |
| Secrets | Vault (Raft storage) | 5 |
| Cache | Dragonfly | 6 |
| Queue | NATS | 6 |
| Storage | Deferred (see Notes) | 6 |
| Ingress | Traefik (Nomad Provider) | 7 |
| Load Balancing | Traefik | 7 |
| Logging | Loki | 8 |
| Metrics | Prometheus | 8 |
| Tracing | Jaeger (optional) | 8 |
| Backup/DR | TBD | 9 |
| Management UI | SSH TUI | 10 |

---

## Notes

### Design Principles

- **No Sudo**: Styx never requires sudo. All data in `~/.styx/`, uses launchd user agents.
- **Tailscale-First**: All networking goes through Tailscale. Transport encryption handled by WireGuard.
- **Simplified Stack**: Removed Consul in favor of Nomad native service discovery + Tailscale DNS.

### Architecture Simplification (Dec 2025)

Removed Consul, TLS certificates, and gossip encryption in favor of simpler architecture:
- **Before**: Nomad + Consul + Vault (Consul backend) + TLS + Gossip + Bootstrap Server
- **After**: Nomad + Vault (Raft backend) + Tailscale

**Rationale**:
- Tailscale already provides WireGuard encryption for all inter-node traffic
- Tailscale MagicDNS provides hostname resolution
- Nomad native service discovery works without Consul
- Vault Raft storage eliminates Consul dependency

### Phase 3 Notes (Simplified)

- Services use `provider = "nomad"` for Nomad native service discovery
- Services accessible via Tailscale hostname + port (e.g., `hostname.ts.net:10080`)
- Port convention: hostPort = containerPort + 10000 (e.g., 80 → 10080)

### Phase 4 Discoveries

- Subnet collision problem: all Macs use same 192.168.64.0/24 vmnet subnet, so direct LAN routing won't work
- Solution: TCP proxy in task driver bridges Tailscale → container network
- Port mapping convention: hostPort = containerPort + 10000 (e.g., 80 → 10080)
- Task driver returns Tailscale MagicDNS hostname (e.g., `fimbulwinter.panthera-frog.ts.net`) in DriverNetwork
- Services registered with Tailscale hostname, accessible from any node on the tailnet

### Phase 5 Notes (Simplified)

- Vault uses integrated Raft storage (no external dependencies)
- Vault auto-initialized with 1 unseal key for simplicity (production would use 5 shares, 3 threshold)
- Nomad-Vault integration uses workload identities (JWT auth) for secure, short-lived tokens
- No TLS certificates needed - Tailscale WireGuard handles encryption

### Phase 6 Discoveries

- **Olric doesn't support ARM64** - Docker images are linux/amd64 only. Replaced with Dragonfly (Redis-compatible)
- **Dragonfly requires explicit memory config** - Must pass `--maxmemory=1gb` to prevent reading system memory
- **NATS works well** - Simple deployment, HTTP monitoring at port 18222

#### S3-Compatible Storage (Deferred)

All evaluated options had significant issues for Apple Containers:
- **SeaweedFS**: Complex multi-component architecture, gRPC port issues
- **MinIO**: Deprecated in 2025, entered maintenance mode
- **Garage**: Requires TOML config file, more setup complexity

Recommendation: Use cloud object storage (S3, GCS, R2) if needed.
