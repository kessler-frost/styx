# Styx

[![Latest Release](https://img.shields.io/github/v/release/kessler-frost/styx)](https://github.com/kessler-frost/styx/releases)
[![License](https://img.shields.io/github/license/kessler-frost/styx)](LICENSE)
[![macOS](https://img.shields.io/badge/platform-macOS-blue)](https://github.com/kessler-frost/styx)

Your personal cloud platform built for macOS. Deploy containers, manage secrets, and scale across multiple machines—no Docker required.

## Requirements

- macOS 26+ (Tahoe) with Apple Silicon
- [Homebrew](https://brew.sh), [Apple Container CLI](https://github.com/apple/container), [Tailscale](https://tailscale.com/download)

## Installation

```bash
curl -fsSL https://styx.leviathan.wtf/install.sh | sh
```

Or from source: `git clone https://github.com/kessler-frost/styx && cd styx && make build-all`

## Quick Start

```bash
# Start server
styx init

# Join additional nodes (optional)
styx init --join <server-tailscale-ip>

# Deploy a workload
nomad job run example/nginx.nomad

# Start platform services (optional)
styx services start
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Your Mac (Apple Silicon)                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────┐    ┌─────────────────────────────────────────────┐    │
│  │  styx   │───▶│              Nomad (Orchestrator)           │    │
│  │   CLI   │    │  ┌─────────────────────────────────────┐    │    │
│  └─────────┘    │  │      Apple Container Driver         │    │    │
│                 │  │  ┌───────────┐  ┌───────────┐       │    │    │
│                 │  │  │ Container │  │ Container │  ...  │    │    │
│                 │  │  │  (nginx)  │  │  (redis)  │       │    │    │
│                 │  │  └───────────┘  └───────────┘       │    │    │
│                 │  └─────────────────────────────────────┘    │    │
│                 └─────────────────────────────────────────────┘    │
│                           │                │                       │
│                           ▼                ▼                       │
│                 ┌─────────────────┐ ┌─────────────────┐           │
│                 │      Vault      │ │    Tailscale    │           │
│                 │    (Secrets)    │ │   (Networking)  │           │
│                 └─────────────────┘ └─────────────────┘           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Core Stack:**
- **Nomad** - Container orchestration and scheduling
- **Apple Containers** - Native macOS lightweight VMs
- **Vault** - Secrets management with workload identity
- **Tailscale** - Encrypted mesh networking
- **Traefik** - Ingress with automatic service discovery

**Platform Services** (optional): NATS, Dragonfly, Prometheus, Loki, Grafana

## How It Works

```
  nomad job run nginx.nomad
           │
           ▼
  ┌─────────────────┐
  │  Nomad Server   │  1. Evaluates job
  └────────┬────────┘
           ▼
  ┌─────────────────┐
  │  Nomad Client   │  2. Allocates on node
  └────────┬────────┘
           ▼
  ┌─────────────────┐
  │ Apple Container │  3. Pulls & starts
  │     Driver      │
  └────────┬────────┘
           ▼
  ┌─────────────────┐
  │   Container     │  4. Running via Tailscale
  └─────────────────┘
```

## Example Use Cases

### Local Dev Environment

```
┌───────────────────────────────────────────────┐
│               Your Mac (Styx)                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│  │ Postgres │ │  Redis   │ │   NATS   │     │
│  └──────────┘ └──────────┘ └──────────┘     │
└─────────────────────┬─────────────────────────┘
                      │ Tailscale
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
   ┌────────┐   ┌─────────┐   ┌────────┐
   │ Laptop │   │ Desktop │   │  iPad  │
   └────────┘   └─────────┘   └────────┘
```

### Multi-Node Cluster

```
                 ┌───────────────────┐
                 │   Tailscale Mesh  │
                 └─────────┬─────────┘
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│   Mac Mini    │ │ MacBook Pro   │ │  Mac Studio   │
│   (Server)    │ │   (Client)    │ │   (Client)    │
│               │ │               │ │               │
│ Nomad+Vault   │ │  ┌─────────┐  │ │  ┌─────────┐  │
│ Traefik       │ │  │   API   │  │ │  │   GPU   │  │
│ Database      │ │  │  Server │  │ │  │  Worker │  │
└───────────────┘ │  └─────────┘  │ │  └─────────┘  │
                  └───────────────┘ └───────────────┘
```

### Self-Hosted Apps

```
Internet ──▶ Tailscale ──▶ Traefik ──┬──▶ /blog   ──▶ Ghost
                                     ├──▶ /api    ──▶ API
                                     └──▶ /files  ──▶ Minio
```

## Example Jobs

See `example/` for sample Nomad jobs: `alpine.nomad`, `nginx.nomad`, `nginx-vault.nomad`

## Uninstall

```bash
styx uninstall
```

## License

Apache 2.0 - See [LICENSE](LICENSE)
