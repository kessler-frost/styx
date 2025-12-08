# Styx

Styx unites your Mac devices into a cohesive fleet for running workloads at any scale.

## Overview

Styx combines Apple Containers with HashiCorp tools to turn any collection of Macs into unified infrastructure:

- **Apple Containers** - Native macOS containerization (lightweight VMs)
- **Nomad** - Workload orchestration and scheduling
- **Vault** - Secrets management with workload identity
- **Tailscale** - Secure mesh networking

## Features

- Apple Containers + Nomad orchestration
- Vault secrets management
- Tailscale mesh networking

## Requirements

- macOS 26+ (Tahoe)
- Apple Silicon
- [Homebrew](https://brew.sh)
- [Apple Container CLI](https://github.com/apple/container)
- [Tailscale](https://tailscale.com/download)

## Installation

### From Source

```bash
git clone https://github.com/kessler-frost/styx
cd styx
make build-all
```

## Quick Start

1. Start server node:
   ```bash
   ./bin/styx init
   ```

2. Join client node:
   ```bash
   ./bin/styx init --join <server-tailscale-ip>
   ```

3. Deploy a workload:
   ```bash
   nomad job run example/nginx.nomad
   ```

## Example Jobs

See the `example/` directory for sample Nomad job specifications:
- `alpine.nomad` - Basic Alpine container
- `nginx.nomad` - Nginx web server
- `nginx-vault.nomad` - Nginx with Vault secrets integration

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
