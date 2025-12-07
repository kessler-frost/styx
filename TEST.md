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

### Test Cases

#### 3.1 Service Registration
```bash
nomad job run example/nginx.nomad
nomad service list
# Expected: Shows nginx service
```

#### 3.2 Service Info
```bash
nomad service info nginx
# Expected: Shows service details with Tailscale hostname and port
```

#### 3.3 Service Accessible
```bash
curl http://localhost:10080
# Expected: Returns nginx welcome page
```

### Phase 3 Notes

**Nomad Native Service Discovery**:
- Services use `provider = "nomad"` in job specs
- Services registered with Nomad, not Consul
- Use `nomad service list` and `nomad service info <name>` to query

**Apple Container Networking**:
- Containers get IPv4 addresses on 192.168.64.x subnet (vmnet)
- Containers are reachable from host via TCP proxy (not directly)
- Port mapping uses host ports exposed via proxy

---

## Phase 4: Networking

### Prerequisites
- Phase 3 tests passing
- Tailscale installed and authenticated on both Macs
- Both Macs on the same Tailnet

### Test Cases

#### 4.1 Tailscale Detection
```bash
# Build and reinitialize styx
make build-all
styx stop && styx init --server
# Expected: Shows "Tailscale connected: <hostname>.ts.net (<ip>)"
```

#### 4.2 TCP Proxy Running
```bash
nomad job run example/nginx.nomad
lsof -i :10080
# Expected: Shows styx task driver listening on port 10080
```

#### 4.3 Local Access via Proxy
```bash
curl http://localhost:10080
# Expected: Returns nginx welcome page
```

#### 4.4 Access via Tailscale Hostname
```bash
curl http://fimbulwinter.panthera-frog.ts.net:10080
# Expected: Returns nginx welcome page (replace with your hostname)
```

#### 4.5 Health Check Working
```bash
nomad service list
nomad job status nginx | grep -A5 "Service Status"
# Expected: Service shows as healthy
```

#### 4.6 Service Registered with Tailscale Hostname
```bash
nomad service info nginx
# Expected: Shows Tailscale MagicDNS hostname (e.g., fimbulwinter.panthera-frog.ts.net)
```

#### 4.7 Cross-Node Communication
```bash
# On Mac A (fimbulwinter): Start nginx
styx init --server
nomad job run example/nginx.nomad

# On Mac B: Join and access
styx join fimbulwinter.panthera-frog.ts.net
curl http://fimbulwinter.panthera-frog.ts.net:10080
# Expected: Returns nginx welcome page
```

### Phase 4 Notes

**Port Mapping Convention**:
- Container port 80 → Host port 10080
- Container port 443 → Host port 10443
- Container port 8080 → Host port 18080
- General rule: host_port = container_port + 10000

**Job Spec Format**:
```hcl
network {
  port "http" {
    static = 10080  # Host port
  }
}

task "myapp" {
  config {
    ports = ["10080:80"]  # hostPort:containerPort
  }
  service {
    provider     = "nomad"    # Nomad native service discovery
    address_mode = "driver"   # Uses Tailscale hostname
    check {
      type = "tcp"
      port = "http"  # Works via proxy
    }
  }
}
```

---

## Phase 5: Security

### Prerequisites
- Phase 4 tests passing
- Vault installed (`mise install vault` or `brew install vault`)

### Test Cases

#### 5.1 Vault Running (Server Mode)
```bash
styx stop && styx init --server
vault status
# Expected: Shows initialized and unsealed
# "Sealed: false"
# "Storage Type: raft"

# Root token saved
cat ~/.styx/secrets/vault-init.json
# Expected: Contains unseal_keys_b64 and root_token
```

#### 5.2 Vault KV Store
```bash
# Set VAULT_ADDR
export VAULT_ADDR=http://127.0.0.1:8200

# Login with root token (for testing)
export VAULT_TOKEN=$(cat ~/.styx/secrets/vault-init.json | jq -r '.root_token')

# Store a secret
vault kv put secret/nginx api_key=test123 db_password=secret456
# Expected: Success

# Retrieve secret
vault kv get secret/nginx
# Expected: Shows api_key and db_password
```

#### 5.3 Nomad-Vault Integration
```bash
# Verify Nomad has Vault configured
nomad agent-info | grep -A5 vault
# Expected: Shows vault configuration

# Policy exists
vault policy read nomad-workloads
# Expected: Shows policy allowing secret access
```

#### 5.4 Job with Vault Secrets
```bash
# First create the secret
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=$(cat ~/.styx/secrets/vault-init.json | jq -r '.root_token')
vault kv put secret/data/nginx api_key=test123 db_password=secret456

# Run job that uses Vault secrets
nomad job run example/nginx-vault.nomad
# Expected: Job deploys successfully

# Verify secrets template rendered
nomad alloc logs <alloc-id>
# Expected: No template rendering errors
```

### Phase 5 Notes

**Security Model**:
- Vault for secrets - protects sensitive data in jobs (Raft storage, no external dependencies)
- Tailscale for transport - encrypts all inter-node traffic via WireGuard
- Nomad workload identities (JWT auth) for Vault integration

**Vault Storage**:
- Uses integrated Raft storage (no Consul dependency)
- Auto-initialized with 1 unseal key for simplicity
- Production deployments should use 5 shares, 3 threshold

---

## Phase 6: Distributed Primitives

### Test Cases

#### 6.1 Dragonfly Cache
```bash
nomad job run example/dragonfly.nomad
nomad service list
# Expected: Shows dragonfly service

# Test Redis-compatible operations
redis-cli -p 16379 SET test "hello"
redis-cli -p 16379 GET test
# Expected: Returns "hello"
```

#### 6.2 NATS Queue
```bash
nomad job run example/nats.nomad
nomad service list
# Expected: Shows nats service

# Check health endpoint
curl http://localhost:18222/healthz
# Expected: Returns ok
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
