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
        image = "nginx:latest"
        ports = ["80:10080"]  # containerPort:hostPort
      }

      resources {
        cpu    = 10
        memory = 32
      }

      service {
        name = "nginx"
        port = "http"
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
