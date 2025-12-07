job "nginx-vault" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    count = 1

    network {
      port "http" {
        static = 10082
      }
    }

    task "nginx" {
      driver = "apple-container"

      service {
        name         = "nginx-vault"
        port         = "http"
        address_mode = "driver"

        # Note: address_mode="driver" for checks only works with Docker driver
        # For external drivers, use host mode with the port label
        check {
          type     = "tcp"
          port     = "http"
          interval = "10s"
          timeout  = "2s"
        }
      }

      # Vault integration using workload identity (Nomad 1.7+)
      vault {
        # Uses default workload identity configured in Nomad server
        # Role is specified to match the Vault JWT auth role
        role = "nomad-workloads"
      }

      config {
        image = "docker.io/library/nginx:alpine"
        ports = ["10082:80"]
      }

      # Template that injects Vault secrets as environment variables
      template {
        data = <<EOF
{{with secret "secret/data/nginx"}}
API_KEY={{.Data.data.api_key}}
DB_PASSWORD={{.Data.data.db_password}}
{{end}}
EOF
        destination = "secrets/env"
        env         = true
      }

      # Template that creates a config file with secrets
      template {
        data = <<EOF
# This file contains secrets from Vault
{{with secret "secret/data/nginx"}}
server_name: nginx-vault
api_key: {{.Data.data.api_key}}
database:
  password: {{.Data.data.db_password}}
{{end}}
EOF
        destination = "local/config.yaml"
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
