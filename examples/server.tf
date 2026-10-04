terraform {
  required_providers {
    berth = {
      source = "tech-arch1tect/berth"
    }
  }
}

provider "berth" {
  url     = "https://berth.example.com"
  api_key = "brth_your_api_key_here"
}

variable "agent_access_token" {
  description = "Access token the server uses to reach the Berth agent"
  type        = string
  sensitive   = true
}

variable "backup_password" {
  description = "Password protecting the server's stack backups"
  type        = string
  sensitive   = true
}

resource "berth_server" "worker" {
  name            = "worker-1"
  host            = "worker-1.example.com"
  port            = 8443
  description     = "Managed by Terraform"
  access_token    = var.agent_access_token
  backup_password = var.backup_password
  backups_enabled = true
  is_active       = true
  s3_bucket_id    = berth_s3_bucket.backups.id
}

output "server_id" {
  description = "Berth ID of the registered server"
  value       = berth_server.worker.id
}
