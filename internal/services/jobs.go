package services

import "strings"

// Embedded HCL job specs for platform services

const natsJobHCL = `job "nats" {
  datacenters = ["dc1"]
  type        = "service"

  group "nats" {
    count = 1

    network {
      port "client" {
        static = 4222
      }
      port "cluster" {
        static = 6222
      }
      port "monitor" {
        static = 8222
      }
    }

    task "nats" {
      driver = "apple-container"

      config {
        image   = "nats:latest"
        network = "styx"
        ports   = ["4222:4222", "6222:6222", "8222:8222"]
        args    = [
          "-cluster", "nats://0.0.0.0:6222",
          "-http_port", "8222",
          "-cluster_name", "styx-nats"
        ]
      }

      resources {
        cpu    = 100
        memory = 64
      }

      service {
        name         = "nats"
        provider     = "nomad"
        port         = "client"
        address_mode = "driver"

        check {
          type     = "http"
          path     = "/healthz"
          port     = "monitor"
          interval = "10s"
          timeout  = "2s"
        }
      }

      service {
        name         = "nats-cluster"
        provider     = "nomad"
        port         = "cluster"
        address_mode = "driver"
      }

      service {
        name         = "nats-monitor"
        provider     = "nomad"
        port         = "monitor"
        address_mode = "driver"
      }
    }
  }
}
`

const dragonflyJobHCL = `job "dragonfly" {
  datacenters = ["dc1"]
  type        = "service"

  group "dragonfly" {
    count = 1

    network {
      port "redis" {
        static = 6379
      }
    }

    task "dragonfly" {
      driver = "apple-container"

      config {
        image   = "docker.dragonflydb.io/dragonflydb/dragonfly:latest"
        network = "styx"
        ports   = ["6379:6379"]
        args    = [
          "--bind", "0.0.0.0",
          "--port", "6379",
          "--maxmemory", "1gb",
          "--logtostderr"
        ]
      }

      resources {
        cpu    = 500
        memory = 1024
      }

      service {
        name         = "dragonfly"
        provider     = "nomad"
        port         = "redis"
        address_mode = "driver"

        check {
          type     = "tcp"
          port     = "redis"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
`

// traefikJobHCLTemplate is the HCL template for Traefik ingress controller.
// It requires the Nomad API address to be substituted via TraefikJobHCL().
const traefikJobHCLTemplate = `job "traefik" {
  datacenters = ["dc1"]
  type        = "service"

  group "traefik" {
    count = 1

    network {
      port "http" {
        static = 4200
      }
      port "dashboard" {
        static = 4201
      }
    }

    task "traefik" {
      driver = "apple-container"

      config {
        image   = "traefik:v3.2"
        network = "styx"
        ports   = ["4200:80", "4201:8080"]
        args    = [
          "--log.level=DEBUG",
          "--entryPoints.http.address=:80",
          "--api.dashboard=true",
          "--api.insecure=true",
          "--ping=true",
          "--providers.nomad=true",
          "--providers.nomad.endpoint.address=http://{{NOMAD_ADDR}}:4646",
          "--providers.nomad.exposedByDefault=true",
          "--providers.nomad.defaultRule=PathPrefix(` + "`" + `/{{ .Name }}` + "`" + `)"
        ]
      }

      resources {
        cpu    = 200
        memory = 256
      }

      service {
        name         = "traefik"
        provider     = "nomad"
        port         = "http"
        address_mode = "driver"

        check {
          type     = "http"
          path     = "/ping"
          port     = "dashboard"
          interval = "10s"
          timeout  = "2s"
        }
      }

      service {
        name         = "traefik-dashboard"
        provider     = "nomad"
        port         = "dashboard"
        address_mode = "driver"
      }
    }
  }
}
`

// TraefikJobHCL returns the Traefik job HCL with the Nomad address substituted.
// nomadAddr should be the Tailscale IP of the host (e.g., "100.97.142.17").
func TraefikJobHCL(nomadAddr string) string {
	return strings.ReplaceAll(traefikJobHCLTemplate, "{{NOMAD_ADDR}}", nomadAddr)
}
