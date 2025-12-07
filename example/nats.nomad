job "nats" {
  datacenters = ["dc1"]
  type        = "service"

  group "nats" {
    # Use count = 1 for single-node testing
    # For HA across multiple nodes, increase count and uncomment constraint below
    count = 1

    # Uncomment for multi-node HA (spreads allocations across distinct nodes)
    # constraint {
    #   operator = "distinct_hosts"
    #   value    = "true"
    # }

    network {
      port "client" {
        static = 14222  # NATS client connections
      }
      port "cluster" {
        static = 16222  # NATS cluster routing
      }
      port "monitor" {
        static = 18222  # HTTP monitoring
      }
    }

    task "nats" {
      driver = "apple-container"

      config {
        image   = "nats:latest"
        network = "styx"
        ports   = ["14222:4222", "16222:6222", "18222:8222"]
        # Note: For multi-node clustering, use Nomad template to resolve service addresses
        # or configure static routes via Tailscale hostnames
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

      # Client connection service
      service {
        name         = "nats"
        provider     = "nomad"  # Nomad native service discovery
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

      # Cluster routing service (for inter-node discovery)
      service {
        name         = "nats-cluster"
        provider     = "nomad"  # Nomad native service discovery
        port         = "cluster"
        address_mode = "driver"
      }

      # Monitoring service
      service {
        name         = "nats-monitor"
        provider     = "nomad"  # Nomad native service discovery
        port         = "monitor"
        address_mode = "driver"
      }
    }
  }
}
