# Styx Testing Requirements

This document outlines the test cases to run after completing each phase of implementation.

---

## Phase 1: Foundation (Apple Container Task Driver)

### Prerequisites
- macOS 26 with Apple Container CLI installed
- Nomad installed (`brew install nomad`)
- Driver built (`make build`)

### Test Cases

#### 1.1 Driver Registration
```bash
make dev
# Expected: Nomad logs show "detected plugin: name=apple-container type=driver"
# Expected: `nomad node status -self` shows "Driver Status = apple-container,docker,raw_exec"
```

#### 1.2 Container Start - Alpine
```bash
nomad job run example/alpine.nomad
# Expected: Deployment status = successful
# Expected: `container list` shows running alpine container
```

#### 1.3 Container Start - Nginx
```bash
nomad job run example/nginx.nomad
# Expected: Deployment status = successful
# Expected: `container list` shows running nginx container
```

#### 1.4 Container Start - Ubuntu
```bash
nomad job run example/ubuntu.nomad
# Expected: Deployment status = successful
# Expected: `container list` shows running ubuntu container
```

#### 1.5 Multiple Containers
```bash
nomad job run example/alpine.nomad
nomad job run example/nginx.nomad
nomad job run example/ubuntu.nomad
container list
# Expected: All 3 containers running simultaneously
```

#### 1.6 Container Stop
```bash
nomad job stop alpine
# Expected: Job stopped, container removed within 10 seconds
# Expected: `container list` no longer shows alpine container
```

#### 1.7 All Jobs Stop
```bash
nomad job stop alpine && nomad job stop nginx && nomad job stop ubuntu
sleep 10
container list
# Expected: No containers running
```

#### 1.8 Health Check
```bash
nomad node status -self | grep "Driver Status"
# Expected: apple-container listed as healthy
```

### Cleanup
```bash
pkill -f "nomad agent"
```

---

## Phase 2: Cluster Basics

### Prerequisites
- Phase 1 tests passing
- Multiple Macs available (or VMs)

### Test Cases

#### 2.1 Styx Init (Server)
```bash
styx init --server
# Expected: Nomad server starts
# Expected: `nomad server members` shows 1 member
```

#### 2.2 Styx Join (Client)
```bash
# On second Mac:
styx join <server-ip>
# Expected: `nomad node status` shows 2 nodes
```

#### 2.3 Job Runs on Client Node
```bash
nomad job run example/alpine.nomad
# Expected: Container starts on client node (not server)
```

#### 2.4 launchd Integration
```bash
styx init --server
# Reboot Mac
# Expected: Styx automatically starts via launchd
```

---

## Phase 3: Service Discovery

### Prerequisites
- Phase 2 tests passing
- Consul installed

### Test Cases

#### 3.1 Consul Agent Running
```bash
consul members
# Expected: Shows cluster members
```

#### 3.2 Service Registration
```bash
nomad job run example/nginx.nomad
consul catalog services
# Expected: Shows nginx service
```

#### 3.3 DNS Resolution
```bash
dig @127.0.0.1 -p 8600 nginx.service.consul
# Expected: Returns container IP
```

#### 3.4 KV Store
```bash
consul kv put config/test "hello"
consul kv get config/test
# Expected: Returns "hello"
```

---

## Phase 4: Networking

### Prerequisites
- Phase 3 tests passing
- Tailscale installed and authenticated

### Test Cases

#### 4.1 Container Gets Tailscale IP
```bash
nomad job run example/nginx.nomad
container inspect <container-id> | grep tailscale
# Expected: Container has Tailscale IP
```

#### 4.2 Cross-Node Communication
```bash
# Start nginx on Node A, alpine on Node B
# From alpine container:
curl http://<nginx-tailscale-ip>
# Expected: Returns nginx welcome page
```

---

## Phase 5: Security

### Test Cases

#### 5.1 mTLS Between Services
```bash
# TBD - Consul Connect integration
```

#### 5.2 Vault Secrets
```bash
# TBD - Vault integration
```

---

## Phase 6: Distributed Primitives

### Test Cases

#### 6.1 Olric Cache
```bash
nomad job run jobs/olric.nomad
# Test cache operations from container
```

#### 6.2 NATS Queue
```bash
nomad job run jobs/nats.nomad
# Test pub/sub from containers
```

#### 6.3 SeaweedFS Storage
```bash
nomad job run jobs/seaweedfs.nomad
# Test S3-compatible operations
```

---

## Phase 7: Ingress

### Test Cases

#### 7.1 Traefik Running
```bash
nomad job run jobs/traefik.nomad
curl http://localhost/
# Expected: Traefik dashboard or routed service
```

#### 7.2 Auto-Discovery
```bash
nomad job run example/nginx.nomad
curl http://nginx.localhost/
# Expected: Returns nginx welcome page
```

---

## Phase 8: Observability

### Test Cases

#### 8.1 Logs Available
```bash
nomad alloc logs <alloc-id>
# Expected: Container logs displayed
```

#### 8.2 Metrics Collection
```bash
# TBD - Prometheus/Loki setup
```

---

## Quick Smoke Test

Run this after any changes to verify basic functionality:

```bash
# Start Nomad
make dev &
sleep 15

# Run test jobs
nomad job run example/alpine.nomad
nomad job run example/nginx.nomad

# Verify running
nomad job status alpine | grep -q "running" && echo "PASS: alpine" || echo "FAIL: alpine"
nomad job status nginx | grep -q "running" && echo "PASS: nginx" || echo "FAIL: nginx"
container list | grep -c "running" | grep -q "2" && echo "PASS: 2 containers" || echo "FAIL: containers"

# Cleanup
nomad job stop alpine
nomad job stop nginx
sleep 10
container list | grep -c "running" | grep -q "0" && echo "PASS: cleanup" || echo "FAIL: cleanup"

pkill -f "nomad agent"
echo "Smoke test complete"
```

---

## Test Matrix

| Image | Start | Stop | Health | Logs |
|-------|-------|------|--------|------|
| alpine:latest | - | - | - | - |
| nginx:latest | - | - | - | - |
| ubuntu:latest | - | - | - | - |
| debian:latest | - | - | - | - |
| python:3 | - | - | - | - |

Mark with checkmarks as tested.
