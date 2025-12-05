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

**Deliverable**: 2+ Macs running as Nomad cluster (COMPLETE)

---

## Phase 3: Service Discovery ✓
**Goal**: Services find each other

- [x] Add Consul to the stack
- [x] Consul DNS for service names
- [x] Consul KV for configuration
- [x] Health checks in Consul (disabled until Phase 4 networking)
- [x] Task driver returns DriverNetwork with container IP
- [x] Service registration with container IP via `address_mode = "driver"`

**Deliverable**: `curl http://myservice.service.consul` works (COMPLETE)

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

- [x] TLS for Nomad/Consul APIs (always enabled)
- [x] Gossip encryption for Consul cluster
- [x] Vault integration for secrets (server mode only)
- [x] Certificate generation using Consul CLI
- [~] Consul Connect for mTLS (NOT SUPPORTED on macOS - sidecars require Linux CNI)

**Note**: Consul Connect sidecars don't work on macOS due to Linux CNI bridge networking requirement.
Tailscale provides encrypted transport between nodes. Consul intentions can be used for authorization policy.

**Deliverable**: TLS for APIs, Vault for secrets, Tailscale for transport encryption (COMPLETE)

---

## Phase 6: Distributed Primitives
**Goal**: Cache, queue, storage available to services

- [ ] Deploy Olric as Nomad job (distributed cache)
- [ ] Deploy NATS as Nomad job (message queue)
- [ ] Deploy SeaweedFS as Nomad job (distributed storage)
- [ ] Example client libraries/configs

**Deliverable**: Services can use redis:// nats:// s3:// endpoints

---

## Phase 7: Ingress & Load Balancing
**Goal**: External traffic reaches services

- [ ] Deploy Traefik as Nomad job
- [ ] Consul Catalog integration (auto-discovery)
- [ ] TLS termination
- [ ] Load balancing across replicas

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
| Service Discovery | Consul | 3 |
| KV/Config | Consul KV | 3 |
| DNS | Consul DNS | 3 |
| Networking | Tailscale | 4 |
| mTLS | Consul Connect | 5 |
| Secrets | Vault | 5 |
| Cache | Olric | 6 |
| Queue | NATS | 6 |
| Storage | SeaweedFS | 6 |
| Ingress | Traefik | 7 |
| Load Balancing | Traefik + Consul | 7 |
| Logging | Loki | 8 |
| Metrics | Prometheus | 8 |
| Tracing | Jaeger (optional) | 8 |
| Backup/DR | TBD | 9 |
| Management UI | SSH TUI | 10 |

---

## Notes

### Phase 3 Discoveries

- Apple Containers get IPv4 addresses on the 192.168.64.x subnet (vmnet)
- Containers are reachable from the host via their container IP, not localhost
- Port mapping (`-p 80:8080`) doesn't expose ports to localhost like Docker
- The task driver must return `DriverNetwork` with the container's IP for proper service registration
- Services must use `address_mode = "driver"` and be defined inside the task block
- Health checks will fail until Phase 4 networking because localhost can't reach containers
- DNS resolver for .consul domain requires `/etc/resolver/consul` with nameserver 127.0.0.1 port 8600

### Phase 4 Discoveries

- Subnet collision problem: all Macs use same 192.168.64.0/24 vmnet subnet, so direct LAN routing won't work
- Solution: TCP proxy in task driver bridges Tailscale → container network
- Port mapping convention: hostPort = containerPort + 10000 (e.g., 80 → 10080)
- Task driver returns Tailscale MagicDNS hostname (e.g., `fimbulwinter.panthera-frog.ts.net`) in DriverNetwork
- Services registered in Consul with Tailscale hostname, accessible from any node on the tailnet
- Health checks work now because they connect via host port proxy
- Tailscale MagicDNS resolves machine names; Consul DNS resolves service names

### Phase 5 Discoveries

- **Consul Connect sidecars do NOT work on macOS** - they require Linux CNI bridge networking (GitHub Issue #12917)
- Alternative security model: Tailscale already encrypts all inter-node traffic (WireGuard), so we add TLS for APIs + Vault
- TLS certificates generated using Consul's built-in CA: `consul tls ca create`, `consul tls cert create`
- Gossip encryption key generated with `consul keygen` and stored in secrets directory
- For client nodes joining: CA and gossip key must be manually copied from server (secure distribution)
- Vault deployed as launchd service (not Nomad job) to avoid chicken-and-egg problem
- Vault uses Consul storage backend for HA
- Vault auto-initialized with 1 unseal key for simplicity (production would use 5 shares, 3 threshold)
- Nomad-Vault integration creates policy and token role for job secrets access
- Consul intentions can be used for service authorization (without sidecar enforcement)
