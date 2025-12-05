job "alpine" {
  datacenters = ["dc1"]
  type        = "service"

  group "test" {
    count = 1

    task "alpine" {
      driver = "apple-container"

      config {
        image   = "alpine:latest"
        command = "sleep"
        args    = ["3600"]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
