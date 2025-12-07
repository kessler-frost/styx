# Example nginx job with explicit Traefik routing tags
# This demonstrates custom routing configuration for Traefik ingress
#
# With this configuration:
# - Access via path: https://hostname.ts.net/nginx-web
# - Access via host: curl -H "Host: nginx.local" https://hostname.ts.net
#
# Note: Uses port 10081 to avoid conflict with Traefik on 10080

job "nginx-traefik" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    count = 1

    network {
      port "http" {
        static = 10081
      }
    }

    task "nginx" {
      driver = "apple-container"

      config {
        image   = "nginx:latest"
        network = "styx"
        ports   = ["10081:80"]
      }

      resources {
        cpu    = 100
        memory = 32
      }

      service {
        name         = "nginx-web"
        provider     = "nomad"
        port         = "http"
        address_mode = "driver"

        # Traefik routing tags
        # These override the default PathPrefix rule for this service
        tags = [
          "traefik.enable=true",
          "traefik.http.routers.nginx-web.rule=Host(`nginx.local`) || PathPrefix(`/nginx-web`)",
          "traefik.http.routers.nginx-web.entrypoints=http",
        ]

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
