job "volumes-demo" {
  datacenters = ["dc1"]
  type        = "service"

  group "demo" {
    count = 1

    task "nginx" {
      driver = "apple-container"

      config {
        image   = "nginx:alpine"
        network = "styx"

        # Volumes support two formats:
        # 1. Named volumes (auto-created): "volume-name:/container/path"
        # 2. Bind mounts: "/host/path:/container/path"
        volumes = [
          "nginx-data:/usr/share/nginx/html",           # Named volume (persists across restarts)
          "nginx-cache:/var/cache/nginx",                # Another named volume
        ]
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }

    task "writer" {
      driver = "apple-container"

      config {
        image   = "alpine:latest"
        network = "styx"
        command = "/bin/sh"
        args = [
          "-c",
          "while true; do echo '<h1>Hello from Styx at '$(date)'</h1>' > /data/index.html; sleep 10; done"
        ]

        # This task shares the same named volume with nginx
        volumes = [
          "nginx-data:/data",  # Same named volume, different mount point
        ]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}
