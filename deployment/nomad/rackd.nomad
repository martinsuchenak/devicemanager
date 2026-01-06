job "rackd" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "devices" {
    count = 1

    network {
      port "http" {
        to = 8080
      }
    }

    volume "data" {
      type            = "host"
      read_only       = false
      source          = "rackd-data"
      attachment_mode = "file-system"
      # Create directory if it doesn't exist
      per_alloc       = true
    }

    service {
      name = "rackd"
      port = "http"

      tags = [
        "urlprefix-/",
        "api",
      ]

      check {
        type     = "http"
        path     = "/api/devices"
        interval = "30s"
        timeout  = "3s"
        header {
          Authorization = ["Bearer ${nomad_var_rackd_bearer_token}"]
        }
      }

      connect {
        sidecar_service {}
      }
    }

    task "server" {
      driver = "docker"

      config {
        image = "ghcr.io/martinsuchenak/rackd:latest"
        args  = ["server"]

        ports = ["http"]

        volumes = [
          "/alloc/data:/app/data",
        ]
      }

      volume_mount {
        volume      = "data"
        destination = "/app/data"
        read_only   = false
      }

      env {
        RACKD_DATA_DIR       = "/app/data"
        RACKD_LISTEN_ADDR    = ":8080"
        RACKD_STORAGE_FORMAT = "json"
      }

      # Optional: Pass bearer token from Nomad vars
      template {
        data        = <<-EOF
          {{ with nomad_var "dm_bearer_token" -}}
          RACKD_BEARER_TOKEN={{ . }}
          {{ end -}}
        EOF
        destination = "secrets/env.txt"
        env         = true
      }

      resources {
        cpu    = 256
        memory = 512
      }

      log_sink {
        type = "file"
        config {
          file_name = "stdout"
        }
      }

      restart_policy = {
        limit = 3
        mode  = "fail"
      }
    }
  }

  # Update strategy
  update {
    max_parallel     = 1
    health_check     = "checks"
    min_healthy_time = "10s"
    healthy_deadline = "3m"
    auto_revert      = true
    auto_promote     = true
    canary           = 1
  }

  # Rollback configuration
  rollback {
    max_parallel     = 1
    health_check     = "checks"
    min_healthy_time = "10s"
    healthy_deadline = "3m"
    auto_revert      = true
  }
}
