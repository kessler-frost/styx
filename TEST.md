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

#### 2.1 Styx Server
```bash
styx -y
# Expected: Nomad server starts
# Expected: `nomad server members` shows 1 member
```

#### 2.2 Styx Join
```bash
# On second Mac (auto-discovers server):
styx
# Expected: `nomad node status` shows 2 nodes
```

#### 2.3 Job Runs on Client Node
```bash
nomad job run example/alpine.nomad
# Expected: Container starts on client node (not server)
```

#### 2.4 launchd Integration
```bash
styx -y
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
curl http://localhost:4200
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
styx stop && styx -y
# Expected: Shows "Tailscale connected: <hostname>.ts.net (<ip>)"
```

#### 4.2 TCP Proxy Running
```bash
nomad job run example/nginx.nomad
lsof -i :4200
# Expected: Shows styx task driver listening on port 4200
```

#### 4.3 Local Access via Proxy
```bash
curl http://localhost:4200
# Expected: Returns nginx welcome page
```

#### 4.4 Access via Tailscale Hostname
```bash
curl http://fimbulwinter.panthera-frog.ts.net:4200
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
# On Mac A (fimbulwinter): Start server and nginx
styx -y
nomad job run example/nginx.nomad

# On Mac B: Join and access
styx
curl http://fimbulwinter.panthera-frog.ts.net:4200
# Expected: Returns nginx welcome page
```

### Phase 4 Notes

**Port Convention**:
Platform services use standard ports where possible:
- NATS: 4222 (client), 6222 (cluster), 8222 (monitor)
- Dragonfly: 6379 (Redis-compatible)
- Traefik: 4200 (HTTP ingress), 4201 (dashboard)

User services can use any available port.

**Job Spec Format**:
```hcl
network {
  port "http" {
    static = 8080  # Any available port
  }
}

task "myapp" {
  config {
    network = "styx"  # Required: shared container network
    ports   = ["8080:80"]  # hostPort:containerPort
  }
  service {
    provider     = "nomad"    # Nomad native service discovery
    address_mode = "driver"   # Uses Tailscale hostname
    check {
      type = "tcp"
      port = "http"
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
styx stop && styx -y
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
redis-cli -p 6379 SET test "hello"
redis-cli -p 6379 GET test
# Expected: Returns "hello"
```

#### 6.2 NATS Queue
```bash
nomad job run example/nats.nomad
nomad service list
# Expected: Shows nats service

# Check health endpoint
curl http://localhost:8222/healthz
# Expected: Returns ok
```

---

## Phase 7: Ingress

### Prerequisites
- Styx server running with `styx init --serve`
- Tailscale connected (for HTTPS ingress)

### Test Cases

#### 7.1 Traefik Platform Service
```bash
# Traefik is auto-deployed with other platform services
styx services
# Expected: traefik shows [running]

# Check Traefik dashboard
curl http://localhost:4201/api/overview
# Expected: JSON response with Traefik stats

# Health check
curl http://localhost:4201/ping
# Expected: "OK"
```

#### 7.2 Tailscale Serve
```bash
# Tailscale Serve should be enabled automatically
tailscale serve status
# Expected: Shows "/" -> http://127.0.0.1:4200

# Access via HTTPS (replace hostname with your Tailscale FQDN)
curl https://$(tailscale status --json | jq -r '.Self.DNSName' | sed 's/\.$//')/
# Expected: 404 (no services routed to /)
```

#### 7.3 Auto-Discovery (Path-Based Routing)
```bash
# Deploy nginx-traefik example
NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad job run example/nginx-traefik.nomad

# Wait for service to register
sleep 10

# Check Traefik discovered it
curl http://localhost:4201/api/http/routers
# Expected: Shows "nginx-web" router

# Access via path prefix (via Traefik on local port)
curl http://localhost:4200/nginx-web
# Expected: nginx welcome page

# Access via HTTPS (replace hostname with your Tailscale FQDN)
HOSTNAME=$(tailscale status --json | jq -r '.Self.DNSName' | sed 's/\.$//')
curl https://$HOSTNAME/nginx-web
# Expected: nginx welcome page
```

#### 7.4 Explicit Routing Tags
```bash
# The nginx-traefik example has explicit tags for Host-based routing
# Access via Host header (localhost)
curl -H "Host: nginx.local" http://localhost:4200
# Expected: nginx welcome page
```

#### 7.5 Load Balancing (Multiple Instances)
```bash
# Edit nginx-traefik.nomad to set count = 2
# Then redeploy and check Traefik sees both backends
NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad job run example/nginx-traefik.nomad

curl http://localhost:4201/api/http/services
# Expected: nginx-web service shows 2 servers
```

#### 7.6 Cleanup
```bash
NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad job stop nginx-traefik
```

---

## Phase 8: Observability

### Prerequisites
- Styx server running with `styx init --serve`
- Phase 7 (Ingress) working

### Test Cases

#### 8.1 Observability Services Running
```bash
styx services
# Expected: All services show [running]:
#   - prometheus
#   - loki
#   - grafana
#   - promtail
#   - traefik
#   - nats
#   - dragonfly
```

#### 8.2 Prometheus Targets
```bash
curl -s http://localhost:9090/prometheus/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
# Expected: All targets show health: "up"
#   - nomad
#   - prometheus
#   - traefik-metrics (via Nomad SD)
```

#### 8.3 Prometheus Metrics Query
```bash
curl -s 'http://localhost:9090/prometheus/api/v1/query?query=up' | jq '.data.result[] | {job: .metric.job, value: .value[1]}'
# Expected: All jobs show value: "1"
```

#### 8.4 Traefik Metrics Endpoint
```bash
curl -s http://localhost:8082/metrics | head -5
# Expected: Prometheus format metrics
```

#### 8.5 Grafana Accessible
```bash
curl -s http://localhost:4200/grafana/api/health
# Expected: {"commit":"...","database":"ok","version":"..."}
```

#### 8.6 Grafana Datasources
```bash
curl -s http://localhost:4200/grafana/api/datasources | jq '.[].name'
# Expected: "Prometheus" and "Loki"
```

#### 8.7 Loki Ready
```bash
curl -s http://localhost:3100/ready
# Expected: "ready"
```

#### 8.8 Nomad Metrics Endpoint
```bash
curl -s 'http://localhost:4646/v1/metrics?format=prometheus' | grep -c nomad_
# Expected: > 0 (many Nomad metrics)
```

#### 8.9 Nomad Logs via CLI
```bash
# Get any running allocation
ALLOC=$(NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad job allocs prometheus -json | jq -r '.[0].ID')
NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad alloc logs $ALLOC
# Expected: Prometheus startup logs
```

#### 8.10 Prometheus via Traefik
```bash
curl -s http://localhost:4200/prometheus/api/v1/status/config | jq '.status'
# Expected: "success"
```

### Phase 8 Notes

**Architecture**:
- Prometheus, Loki, Grafana run only on server nodes (`node.class = "server"`)
- Promtail runs on ALL nodes as a system job to ship logs
- Prometheus auto-discovers services tagged with `prometheus.scrape=true`

**Endpoints** (via Traefik at localhost:4200):
- Grafana: `/grafana`
- Prometheus: `/prometheus`

**Direct Access**:
- Prometheus: `http://localhost:9090`
- Loki: `http://localhost:3100`
- Grafana: `http://localhost:3000`
- Traefik metrics: `http://localhost:8082/metrics`

---

## Phase 9: Complete Platform & Hardening

### Prerequisites
- Styx server running with `styx init --serve`
- Phase 8 (Observability) working
- Vault running and initialized

### Test Cases

#### 9.1 PostgreSQL Platform Service
```bash
styx services
# Expected: postgres shows [running]

# Check PostgreSQL is responding (via styx network)
# Get postgres container IP
POSTGRES_IP=$(container list --format json | jq -r '.[] | select(.configuration.id | contains("postgres")) | .networks[0].address' | cut -d'/' -f1)
container exec $(container list --format json | jq -r '.[0].configuration.id') -- psql -h $POSTGRES_IP -U styx -d styx -c "SELECT 1"
# Expected: Returns "1"
```

#### 9.2 RustFS Platform Service
```bash
styx services
# Expected: rustfs shows [running]

# Check RustFS health
RUSTFS_IP=$(container list --format json | jq -r '.[] | select(.configuration.id | contains("rustfs")) | .networks[0].address' | cut -d'/' -f1)
echo "RustFS at $RUSTFS_IP"
# Expected: Shows IP address
```

#### 9.3 Chaos Testing - Agent Recovery
```bash
styx chaos --agent
# Expected:
#   - Nomad agent stops
#   - Agent restarts automatically via launchd
#   - Agent becomes healthy again
```

#### 9.4 Chaos Testing - Service Recovery
```bash
styx chaos --services
# Expected:
#   - Platform service stops
#   - Nomad restarts the service
#   - Service becomes healthy again
```

#### 9.5 Native Container Volumes
```bash
# Check volumes are being created
container volume ls
# Expected: Shows postgres-data and rustfs-data volumes

# Verify PostgreSQL uses named volume
container inspect $(container list --format json | jq -r '.[] | select(.configuration.id | contains("postgres")) | .configuration.id') | jq '.configuration.mounts'
# Expected: Shows mount with "postgres-data" volume
```

#### 9.6 Container Stats Monitoring
```bash
# Get any running allocation
ALLOC=$(NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad job allocs traefik -json | jq -r '.[0].ID')
NOMAD_ADDR=https://127.0.0.1:4646 NOMAD_SKIP_VERIFY=true nomad alloc status $ALLOC | grep -A10 "Task Resources"
# Expected: Shows CPU and Memory usage stats
```

#### 9.7 Disk Usage Monitoring
```bash
styx system df
# Expected: Shows disk usage for images, containers, and volumes
# Example output:
#   Images:
#     Total:       21
#     Active:      10
#     Size:        5.7 GB
#     Reclaimable: 3.1 GB
```

#### 9.8 Image Prune
```bash
# Show current usage
styx system df

# Prune unused images
styx system prune
# Expected: "Freed X of disk space"

# Verify space was freed
styx system df
# Expected: Lower size/reclaimable values for images
```

#### 9.9 Styx Status Command
```bash
styx status
# Expected: Shows comprehensive status including:
#   - Service status (running/stopped)
#   - Vault health
#   - Nomad health
#   - Mode (server/client)
#   - Cluster members (if server)
#   - Core Services endpoints
#   - Platform Endpoints
```

#### 9.10 Volume Persistence Test
```bash
# Stop postgres
styx services stop postgres

# Restart postgres
styx services start postgres

# Wait for it to start
sleep 10

# Check data persisted (volume should retain data)
container volume ls | grep postgres-data
# Expected: Volume still exists
```

### Phase 9 Notes

**Platform Services**:
- PostgreSQL: Database on port 5432, credentials from Vault
- RustFS: S3-compatible storage on port 9000/9001, credentials from Vault

**Native Container Volumes**:
- `volumes` field supports both bind mounts and named volumes
- Bind mount: `/host/path:/container/path`
- Named volume: `volume-name:/container/path` (auto-created)

**Chaos Testing**:
- `styx chaos --agent`: Tests agent restart recovery
- `styx chaos --services`: Tests service restart via Nomad
- `styx chaos --container`: Tests container CLI availability
- `styx chaos --rejoin`: Tests cluster membership
- `styx chaos --all`: Runs all chaos tests

**Disk Management**:
- `styx system df`: Show container/image/volume disk usage
- `styx system prune`: Remove unused images

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
