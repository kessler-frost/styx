package services

import "strings"

// Embedded HCL job specs for platform services

// NATS job - no host ports, accessed via Traefik TCP routing
const natsJobHCL = `job "nats" {
  datacenters = ["dc1"]
  type        = "service"

  group "nats" {
    count = 1

    task "nats" {
      driver = "apple-container"

      config {
        image   = "nats:latest"
        network = "styx"
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
        port         = 4222
        address_mode = "driver"

        tags = [
          "traefik.enable=true",
          "traefik.tcp.routers.nats.rule=HostSNI(` + "`*`" + `)",
          "traefik.tcp.routers.nats.entrypoints=nats",
          "traefik.tcp.services.nats.loadbalancer.server.port=4222"
        ]
      }

      service {
        name         = "nats-cluster"
        provider     = "nomad"
        port         = 6222
        address_mode = "driver"
      }

      service {
        name         = "nats-monitor"
        provider     = "nomad"
        port         = 8222
        address_mode = "driver"

        tags = [
          "traefik.enable=true",
          "traefik.http.routers.nats-monitor.rule=PathPrefix(` + "`/nats`" + `)",
          "traefik.http.routers.nats-monitor.entrypoints=http"
        ]
        // Note: NATS doesn't expose /metrics - would need nats-prometheus-exporter
      }
    }
  }
}
`

// Dragonfly job - no host ports, accessed via Traefik TCP routing
const dragonflyJobHCL = `job "dragonfly" {
  datacenters = ["dc1"]
  type        = "service"

  group "dragonfly" {
    count = 1

    task "dragonfly" {
      driver = "apple-container"

      config {
        image   = "docker.dragonflydb.io/dragonflydb/dragonfly:latest"
        network = "styx"
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
        port         = 6379
        address_mode = "driver"

        tags = [
          "traefik.enable=true",
          "traefik.tcp.routers.redis.rule=HostSNI(` + "`*`" + `)",
          "traefik.tcp.routers.redis.entrypoints=redis",
          "traefik.tcp.services.redis.loadbalancer.server.port=6379"
        ]
      }
    }
  }
}
`

// traefikJobHCLTemplate is the HCL template for Traefik ingress controller.
// Traefik is the ONLY service with host port exposure - all traffic goes through it.
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
      port "nats" {
        static = 4222
      }
      port "redis" {
        static = 6379
      }
      port "metrics" {
        static = 8082
      }
    }

    task "traefik" {
      driver = "apple-container"

      config {
        image   = "traefik:v3.2"
        network = "styx"
        ports   = ["4200:80", "4201:8080", "4222:4222", "6379:6379", "8082:8082"]
        args    = [
          "--log.level=DEBUG",
          "--entryPoints.http.address=:80",
          "--entryPoints.nats.address=:4222",
          "--entryPoints.redis.address=:6379",
          "--api.dashboard=true",
          "--api.insecure=true",
          "--ping=true",
          "--metrics.prometheus=true",
          "--metrics.prometheus.entryPoint=metrics",
          "--entryPoints.metrics.address=:8082",
          "--providers.nomad=true",
          "--providers.nomad.endpoint.address=http://{{NOMAD_ADDR}}:4646",
          "--providers.nomad.exposedByDefault=false"
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

      service {
        name         = "traefik-metrics"
        provider     = "nomad"
        port         = 8082
        address_mode = "driver"
        tags         = ["prometheus.scrape=true"]
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

// prometheusJobHCLTemplate is the HCL template for Prometheus monitoring.
const prometheusJobHCLTemplate = `job "prometheus" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${node.class}"
    value     = "server"
  }

  group "prometheus" {
    count = 1

    network {
      port "http" {
        static = 9090
      }
    }

    task "prometheus" {
      driver = "apple-container"

      config {
        image   = "prom/prometheus:latest"
        network = "styx"
        ports   = ["9090:9090"]
        args    = [
          "--config.file=/local/prometheus.yml",
          "--storage.tsdb.path=/prometheus",
          "--web.enable-lifecycle",
          "--web.external-url=/prometheus/",
          "--web.route-prefix=/prometheus/"
        ]
      }

      template {
        data = <<EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    metrics_path: '/prometheus/metrics'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'nomad'
    metrics_path: '/v1/metrics'
    params:
      format: ['prometheus']
    static_configs:
      - targets: ['{{NOMAD_ADDR}}:4646']

  # Auto-discover services tagged with prometheus.scrape=true
  - job_name: 'nomad-services'
    nomad_sd_configs:
      - server: 'http://{{NOMAD_ADDR}}:4646'
    relabel_configs:
      # Only scrape services tagged with prometheus.scrape=true
      - source_labels: [__meta_nomad_tags]
        regex: '.*,prometheus\.scrape=true,.*'
        action: keep
      # Use service name as job label
      - source_labels: [__meta_nomad_service]
        target_label: job
      # Use node ID as instance label
      - source_labels: [__meta_nomad_node_id]
        target_label: node
EOF
        destination = "local/prometheus.yml"
      }

      resources {
        cpu    = 200
        memory = 256
      }

      service {
        name         = "prometheus"
        provider     = "nomad"
        port         = 9090
        address_mode = "driver"

        tags = [
          "traefik.enable=true",
          "traefik.http.routers.prometheus.rule=PathPrefix(` + "`" + `/prometheus` + "`" + `)",
          "traefik.http.routers.prometheus.entrypoints=http"
        ]
      }
    }
  }
}
`

// PrometheusJobHCL returns the Prometheus job HCL with the Nomad address substituted.
// nomadAddr should be the Tailscale IP of the host (e.g., "100.97.142.17").
func PrometheusJobHCL(nomadAddr string) string {
	return strings.ReplaceAll(prometheusJobHCLTemplate, "{{NOMAD_ADDR}}", nomadAddr)
}

// lokiJobHCL is the HCL for Loki log aggregation.
const lokiJobHCL = `job "loki" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${node.class}"
    value     = "server"
  }

  group "loki" {
    count = 1

    network {
      port "http" {
        static = 3100
      }
    }

    task "loki" {
      driver = "apple-container"

      config {
        image   = "grafana/loki:latest"
        network = "styx"
        ports   = ["3100:3100"]
        args    = ["-config.file=/etc/loki/local-config.yaml"]
      }

      resources {
        cpu    = 200
        memory = 256
      }

      service {
        name         = "loki"
        provider     = "nomad"
        port         = 3100
        address_mode = "driver"
      }
    }
  }
}
`

// grafanaJobHCL is the HCL for Grafana visualization.
const grafanaJobHCL = `job "grafana" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${node.class}"
    value     = "server"
  }

  group "grafana" {
    count = 1

    network {
      port "http" {
        static = 3000
      }
    }

    task "grafana" {
      driver = "apple-container"

      config {
        image   = "grafana/grafana:latest"
        network = "styx"
        ports   = ["3000:3000"]
        env     = {
          "GF_SERVER_ROOT_URL"            = "http://localhost:4200/grafana/"
          "GF_SERVER_SERVE_FROM_SUB_PATH" = "true"
          "GF_SECURITY_ADMIN_PASSWORD"    = "admin"
          "GF_AUTH_ANONYMOUS_ENABLED"     = "true"
          "GF_AUTH_ANONYMOUS_ORG_ROLE"    = "Viewer"
          "GF_PATHS_PROVISIONING"         = "/local/provisioning"
        }
      }

      template {
        data = <<EOF
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus.service.nomad:9090
    isDefault: true
  - name: Loki
    type: loki
    access: proxy
    url: http://loki.service.nomad:3100
EOF
        destination = "local/provisioning/datasources/datasources.yaml"
      }

      resources {
        cpu    = 200
        memory = 256
      }

      service {
        name         = "grafana"
        provider     = "nomad"
        port         = 3000
        address_mode = "driver"

        tags = [
          "traefik.enable=true",
          "traefik.http.routers.grafana.rule=PathPrefix(` + "`" + `/grafana` + "`" + `)",
          "traefik.http.routers.grafana.entrypoints=http",
          "traefik.http.middlewares.grafana-strip.stripprefix.prefixes=/grafana",
          "traefik.http.routers.grafana.middlewares=grafana-strip"
        ]
      }
    }
  }
}
`

// promtailJobHCLTemplate is the HCL template for Promtail log collector.
// Promtail runs as a system job (one per node) to collect logs from all allocations.
const promtailJobHCLTemplate = `job "promtail" {
  datacenters = ["dc1"]
  type        = "system"

  group "promtail" {
    task "promtail" {
      driver = "apple-container"

      config {
        image   = "grafana/promtail:latest"
        network = "styx"
        args    = ["-config.file=/local/promtail.yaml"]
        volumes = [
          "{{NOMAD_ALLOC_DIR}}:/nomad-logs:ro"
        ]
      }

      template {
        data = <<EOF
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki.service.nomad:3100/loki/api/v1/push

scrape_configs:
  - job_name: nomad-allocs
    static_configs:
      - targets:
          - localhost
        labels:
          job: nomad-allocs
          __path__: /nomad-logs/*/alloc/logs/*.stdout.*

  - job_name: nomad-allocs-stderr
    static_configs:
      - targets:
          - localhost
        labels:
          job: nomad-allocs
          stream: stderr
          __path__: /nomad-logs/*/alloc/logs/*.stderr.*
EOF
        destination = "local/promtail.yaml"
      }

      resources {
        cpu    = 100
        memory = 128
      }

      service {
        name         = "promtail"
        provider     = "nomad"
        port         = 9080
        address_mode = "driver"
      }
    }
  }
}
`

// PromtailJobHCL returns the Promtail job HCL with the Nomad alloc directory substituted.
// nomadAllocDir should be the path to the Nomad alloc directory (e.g., "/opt/nomad/data/alloc").
func PromtailJobHCL(nomadAllocDir string) string {
	return strings.ReplaceAll(promtailJobHCLTemplate, "{{NOMAD_ALLOC_DIR}}", nomadAllocDir)
}

