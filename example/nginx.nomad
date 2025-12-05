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

    service {
      name = "nginx"
      port = "http"

      check {
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "2s"
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
    }
  }
}
