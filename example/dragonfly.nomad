job "dragonfly" {
  datacenters = ["dc1"]
  type        = "service"

  group "dragonfly" {
    # Use count = 1 for single-node testing
    # For HA across multiple nodes, increase count and uncomment constraint below
    count = 1

    # Uncomment for multi-node HA (spreads allocations across distinct nodes)
    # constraint {
    #   operator = "distinct_hosts"
    #   value    = "true"
    # }

    network {
      port "redis" {
        static = 16379  # Redis-compatible client connections
      }
    }

    task "dragonfly" {
      driver = "apple-container"

      config {
        image = "docker.dragonflydb.io/dragonflydb/dragonfly:latest"
        ports = ["16379:6379"]
        args  = [
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

      # Redis-compatible client service
      service {
        name         = "dragonfly"
        provider     = "nomad"  # Nomad native service discovery
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
