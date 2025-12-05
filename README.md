# Styx

A distributed system platform for Mac fleets using Apple Containers and HashiCorp Nomad.

## Overview

Styx makes it easy to run distributed workloads across multiple Apple Silicon Macs. It combines:

- **Apple Containers** - Native macOS containerization (micro-VMs)
- **Nomad** - Workload orchestration
- **Consul** - Service discovery and configuration
- **Tailscale** - Mesh networking

## Requirements

- macOS 26 (Tahoe) or later
- Apple Silicon Mac
- [Apple Container CLI](https://github.com/apple/container) installed
- Tailscale installed and connected

## Status

Work in progress. See [PLAN.md](PLAN.md) for implementation status.

## License

MIT
