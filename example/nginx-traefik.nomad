# Example nginx job with Traefik routing
# All traffic goes through Traefik - no host port exposure needed
#
# Access via: https://hostname.ts.net/nginx-web
# Or with Host header: curl -H "Host: nginx.local" https://hostname.ts.net

job "nginx-traefik" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    count = 1

    task "nginx" {
      driver = "apple-container"

      config {
        image   = "nginx:latest"
        network = "styx"
        # No ports - container only accessible via Traefik
      }

      resources {
        cpu    = 100
        memory = 32
      }

      service {
        name         = "nginx-web"
        provider     = "nomad"
        port         = 80  # Container port - Traefik routes directly to container IP
        address_mode = "driver"

        tags = [
          "traefik.enable=true",
          "traefik.http.routers.nginx-web.rule=Host(`nginx.local`) || PathPrefix(`/nginx-web`)",
          "traefik.http.routers.nginx-web.entrypoints=http",
        ]
        # No Nomad health check - Traefik handles health checking for backend services
      }
    }
  }
}
