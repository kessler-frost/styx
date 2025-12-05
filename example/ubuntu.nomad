job "ubuntu" {
  datacenters = ["dc1"]
  type        = "service"

  group "test" {
    count = 1

    task "ubuntu" {
      driver = "apple-container"

      config {
        image   = "ubuntu:latest"
        command = "sleep"
        args    = ["3600"]
      }

      resources {
        cpu    = 10
        memory = 32
      }
    }
  }
}
