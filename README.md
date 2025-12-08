# Styx

[![Latest Release](https://img.shields.io/github/v/release/kessler-frost/styx)](https://github.com/kessler-frost/styx/releases)
[![License](https://img.shields.io/github/license/kessler-frost/styx)](LICENSE)
[![macOS](https://img.shields.io/badge/platform-macOS-blue)](https://github.com/kessler-frost/styx)

Your personal cloud platform built for macOS.

## Overview

Run your own cloud infrastructure on macOS. Styx brings together the best-in-class tools to create a complete platform:

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

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/kessler-frost/styx/main/install.sh | sh
```

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

## Uninstall

To completely remove styx and its data:

```bash
styx uninstall
```

This will interactively prompt you about removing dependencies installed via Homebrew.

## Example Jobs

See the `example/` directory for sample Nomad job specifications:
- `alpine.nomad` - Basic Alpine container
- `nginx.nomad` - Nginx web server
- `nginx-vault.nomad` - Nginx with Vault secrets integration

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
