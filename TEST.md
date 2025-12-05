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

## Phase 3: Service Discovery ✓

### Prerequisites
- Phase 2 tests passing
- Consul installed (`mise install consul` or `brew install consul`)

### Test Cases

#### 3.1 Consul Agent Running ✓
```bash
consul members
# Expected: Shows cluster members
```

#### 3.2 Service Registration ✓
```bash
nomad job run example/nginx.nomad
consul catalog services
# Expected: Shows nginx service
```

#### 3.3 DNS Resolution ✓
```bash
dig @127.0.0.1 -p 8600 nginx.service.consul +short
# Expected: Returns container IP (e.g., 192.168.64.x)
```

#### 3.4 KV Store ✓
```bash
consul kv put config/test "hello"
consul kv get config/test
# Expected: Returns "hello"
```

#### 3.5 Service Accessible via DNS ✓
```bash
curl http://nginx.service.consul:80/
# Expected: Returns nginx welcome page
```

### Phase 3 Notes

**Important**: For service DNS to work correctly:
1. Services must be defined inside the `task` block (not group level)
2. Services must use `address_mode = "driver"` to register with container IP
3. The task driver returns `DriverNetwork` with the container's IPv4 address
4. DNS resolver must be configured: `/etc/resolver/consul` with `nameserver 127.0.0.1` and `port 8600`

**Apple Container Networking**:
- Containers get IPv4 addresses on 192.168.64.x subnet (vmnet)
- Containers are reachable from host via container IP, NOT localhost
- Port mapping (`-p 80:8080`) does NOT expose to localhost like Docker
- Health checks disabled until Phase 4 (localhost can't reach container ports)

---

## Phase 4: Networking ✓

### Prerequisites
- Phase 3 tests passing
- Tailscale installed and authenticated on both Macs
- Both Macs on the same Tailnet

### Test Cases

#### 4.1 Tailscale Detection ✓
```bash
# Build and reinitialize styx
make build-all
styx stop && styx init --server
# Expected: Shows "Tailscale connected: <hostname>.ts.net (<ip>)"
```

#### 4.2 TCP Proxy Running ✓
```bash
nomad job run example/nginx.nomad
lsof -i :10080
# Expected: Shows styx task driver listening on port 10080
```

#### 4.3 Local Access via Proxy ✓
```bash
curl http://localhost:10080
# Expected: Returns nginx welcome page
```

#### 4.4 Access via Tailscale Hostname ✓
```bash
curl http://fimbulwinter.panthera-frog.ts.net:10080
# Expected: Returns nginx welcome page (replace with your hostname)
```

#### 4.5 Health Check Working ✓
```bash
consul catalog services
nomad job status nginx | grep -A5 "Service Status"
# Expected: Service shows as healthy
```

#### 4.6 Service Registered with Tailscale Hostname ✓
```bash
dig @127.0.0.1 -p 8600 nginx.service.consul +short
# Expected: Returns Tailscale MagicDNS name (e.g., fimbulwinter.panthera-frog.ts.net)
```

#### 4.7 Cross-Node Communication
```bash
# On Mac A (fimbulwinter): Start nginx
styx init --server
nomad job run example/nginx.nomad

# On Mac B (styx): Join and access
styx join fimbulwinter.panthera-frog.ts.net
curl http://fimbulwinter.panthera-frog.ts.net:10080
# Expected: Returns nginx welcome page

curl http://nginx.service.consul:10080
# Expected: Returns nginx welcome page via Consul DNS
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
    ports = ["80:10080"]  # containerPort:hostPort
  }
  service {
    address_mode = "driver"  # Uses Tailscale hostname
    check {
      type = "tcp"
      port = "http"  # Works via proxy
    }
  }
}
```

---

## Phase 5: Security ✓

### Prerequisites
- Phase 4 tests passing
- Vault installed (`mise install vault` or `brew install vault`)

### Test Cases

#### 5.1 TLS for Consul API ✓
```bash
# Reinitialize with TLS (now always enabled)
styx stop && styx init --server

# HTTP still works (for backward compat)
curl http://127.0.0.1:8500/v1/status/leader
# Expected: Returns cluster leader

# HTTPS works
curl -k https://127.0.0.1:8501/v1/status/leader
# Expected: Returns cluster leader

# Verify TLS is configured
consul info | grep encrypt
# Expected: Shows "encrypt = true"
```

#### 5.2 Gossip Encryption ✓
```bash
# Check gossip encryption is enabled
consul info | grep encrypt
# Expected: Shows "encrypt = true"

# Key file exists
ls ~/Library/Application\ Support/styx/secrets/gossip.key
# Expected: File exists
```

#### 5.3 Certificate Generation ✓
```bash
# Check certificates exist
ls ~/Library/Application\ Support/styx/certs/
# Expected: consul-agent-ca.pem, dc1-server-consul-*.pem, dc1-server-consul-*-key.pem
```

#### 5.4 Vault Running (Server Mode) ✓
```bash
vault status
# Expected: Shows initialized and unsealed
# "Sealed: false"

# Root token saved
cat ~/Library/Application\ Support/styx/secrets/vault-init.json
# Expected: Contains unseal_keys_b64 and root_token
```

#### 5.5 Vault KV Store ✓
```bash
# Set VAULT_ADDR
export VAULT_ADDR=http://127.0.0.1:8200

# Login with root token (for testing)
export VAULT_TOKEN=$(cat ~/Library/Application\ Support/styx/secrets/vault-init.json | jq -r '.root_token')

# Store a secret
vault kv put secret/nginx api_key=test123 db_password=secret456
# Expected: Success

# Retrieve secret
vault kv get secret/nginx
# Expected: Shows api_key and db_password
```

#### 5.6 Nomad-Vault Integration ✓
```bash
# Verify Nomad token exists
cat ~/Library/Application\ Support/styx/secrets/nomad-vault-token
# Expected: Shows Vault token for Nomad

# Policy exists
vault policy read nomad-server
# Expected: Shows policy allowing secret access
```

#### 5.7 Job with Vault Secrets ✓
```bash
# First create the secret
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=$(cat ~/Library/Application\ Support/styx/secrets/vault-init.json | jq -r '.root_token')
vault kv put secret/nginx api_key=test123 db_password=secret456

# Run job that uses Vault secrets
nomad job run example/nginx-vault.nomad
# Expected: Job deploys successfully

# Verify secrets template rendered
nomad alloc logs <alloc-id>
# Expected: No template rendering errors
```

#### 5.8 Client Join with TLS
```bash
# On server (Mac A)
styx stop && styx init --server

# Copy CA and gossip key to client (Mac B)
scp ~/Library/Application\ Support/styx/certs/consul-agent-ca.pem macb:~/Library/Application\ Support/styx/certs/
scp ~/Library/Application\ Support/styx/secrets/gossip.key macb:~/Library/Application\ Support/styx/secrets/

# On client (Mac B)
styx join <server-tailscale-ip>
# Expected: Joins cluster with TLS

# Verify cluster membership
consul members
# Expected: Shows both nodes
```

### Phase 5 Notes

**Consul Connect Limitation**:
- Consul Connect sidecars do NOT work on macOS (requires Linux CNI bridge networking)
- See: https://github.com/hashicorp/nomad/issues/12917
- Alternative: Tailscale provides WireGuard encryption for all inter-node traffic

**Security Model**:
- TLS for Nomad/Consul APIs - protects control plane
- Gossip encryption for Consul - protects cluster communication
- Vault for secrets - protects sensitive data in jobs
- Tailscale for transport - encrypts all inter-node traffic

**Certificate Distribution**:
- CA and gossip key must be manually copied from server to clients
- Client certs are generated locally using the shared CA

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
