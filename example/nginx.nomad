job "nginx" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    count = 1

    network {
      port "http" {
        static = 8080
      }
    }

    task "nginx" {
      driver = "apple-container"

      config {
        image = "nginx:latest"
        ports = ["80:8080"]
      }

      resources {
        cpu    = 10
        memory = 32
      }

      service {
        name = "nginx"
        port = "http"
        address_mode = "driver"
        # Health check disabled until Phase 4 networking
        # (Apple Containers don't expose ports to localhost)
      }
    }
  }
}
