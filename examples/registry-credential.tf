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

variable "registry_password" {
  description = "Password for the private registry account"
  type        = string
  sensitive   = true
}

resource "berth_registry_credential" "internal" {
  server_id     = 1
  registry_url  = "registry.internal.example.com:5000"
  username      = "deploy"
  password      = var.registry_password
  stack_pattern = "prod-*"
}

output "registry_credential_id" {
  description = "Berth ID of the registry credential"
  value       = berth_registry_credential.internal.id
}
