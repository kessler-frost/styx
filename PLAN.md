# Styx Implementation Plan

> A distributed system platform for Mac fleets using Apple Containers + Nomad

## Status Legend
- [ ] Not started
- [~] In progress
- [x] Complete

---

## Phase 1: Foundation
**Goal**: Run Apple Containers via Nomad locally

- [ ] Create Nomad task driver skeleton
- [ ] Implement container CLI wrapper
- [ ] Basic lifecycle: start, stop, destroy
- [ ] Parse container inspect output
- [ ] Test with `nomad agent -dev`

**Deliverable**: `nomad job run` launches an Apple Container

---

## Phase 2: Cluster Basics
**Goal**: Multiple Macs form a cluster

- [ ] Styx launcher CLI skeleton (cobra)
- [ ] Download/manage Nomad binary
- [ ] Generate Nomad server/client configs
- [ ] launchd service integration
- [ ] `styx init` / `styx join` commands

**Deliverable**: 2+ Macs running as Nomad cluster

---

## Phase 3: Service Discovery
**Goal**: Services find each other

- [ ] Add Consul to the stack
- [ ] Consul DNS for service names
- [ ] Consul KV for configuration
- [ ] Health checks in Consul

**Deliverable**: `curl http://myservice.service.consul` works

---

## Phase 4: Networking
**Goal**: Containers communicate across Macs via Tailscale

- [ ] Configure container networking with Tailscale IPs
- [ ] Register Tailscale IPs in Consul
- [ ] Cross-node container communication

**Deliverable**: Container on Mac A talks to container on Mac B

---

## Phase 5: Security
**Goal**: Secure communication and secrets

- [ ] Consul Connect for mTLS (service mesh)
- [ ] Vault integration for secrets
- [ ] TLS for Nomad/Consul APIs

**Deliverable**: All inter-service traffic encrypted

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

_Add implementation notes, discoveries, and changes here as you go._
