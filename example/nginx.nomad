job "nginx" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    count = 1

    network {
      port "http" {
        static = 10080  # Host port exposed via Tailscale
      }
    }

    task "nginx" {
      driver = "apple-container"

      config {
        image   = "nginx:latest"
        network = "styx"
        ports   = ["10080:80"]  # hostPort:containerPort
      }

      resources {
        cpu    = 100
        memory = 32
      }

      service {
        name         = "nginx"
        provider     = "nomad"  # Nomad native service discovery
        port         = "http"
        address_mode = "driver"  # Uses Tailscale hostname from DriverNetwork

        check {
          type     = "tcp"
          port     = "http"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
