job "nginx" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    count = 1

    task "nginx" {
      driver = "apple-container"

      config {
        image = "nginx:latest"
        ports = ["80:8080"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
