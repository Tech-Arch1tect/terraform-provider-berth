terraform {
  required_providers {
    berth = {
      source  = "registry.terraform.io/tech-arch1tect/berth"
      version = "0.1.0"
    }
  }
}

provider "berth" {
  url                  = "https://localhost:4443"
  api_key              = var.berth_api_key
  insecure_skip_verify = true
}

variable "berth_api_key" {
  description = "Berth API key"
  type        = string
  sensitive   = true
}

# Define role configurations with their permissions
variable "role_configs" {
  description = "Map of roles and their permission configurations"
  type = map(object({
    description = string
    permissions = list(object({
      server_id       = number
      permission_name = string
      stack_pattern   = string
    }))
  }))

  default = {
    developers = {
      description = "Development team with access to dev stacks"
      permissions = [
        { server_id = 1, permission_name = "stacks.read", stack_pattern = "dev-*" },
        { server_id = 1, permission_name = "stacks.manage", stack_pattern = "dev-*" },
        { server_id = 1, permission_name = "files.read", stack_pattern = "dev-*" },
        { server_id = 1, permission_name = "files.write", stack_pattern = "dev-*" },
        { server_id = 1, permission_name = "logs.read", stack_pattern = "dev-*" }
      ]
    }

    operations = {
      description = "Operations team with full access"
      permissions = [
        { server_id = 1, permission_name = "stacks.read", stack_pattern = "*" },
        { server_id = 1, permission_name = "stacks.manage", stack_pattern = "*" },
        { server_id = 1, permission_name = "files.read", stack_pattern = "*" },
        { server_id = 1, permission_name = "files.write", stack_pattern = "*" },
        { server_id = 1, permission_name = "logs.read", stack_pattern = "*" },
        { server_id = 2, permission_name = "stacks.read", stack_pattern = "*" },
        { server_id = 2, permission_name = "stacks.manage", stack_pattern = "*" }
      ]
    }

    qa_team = {
      description = "QA team with read-only access to staging"
      permissions = [
        { server_id = 1, permission_name = "stacks.read", stack_pattern = "staging-*" },
        { server_id = 1, permission_name = "logs.read", stack_pattern = "staging-*" }
      ]
    }

    readonly = {
      description = "Read-only access to all environments"
      permissions = [
        { server_id = 1, permission_name = "stacks.read", stack_pattern = "*" },
        { server_id = 1, permission_name = "logs.read", stack_pattern = "*" }
      ]
    }
  }
}

# Create all roles with inline permissions using for_each
resource "berth_role" "teams" {
  for_each = var.role_configs

  name        = each.key
  description = each.value.description

  dynamic "permissions" {
    for_each = each.value.permissions
    content {
      server_id       = permissions.value.server_id
      permission_name = permissions.value.permission_name
      stack_pattern   = permissions.value.stack_pattern
    }
  }
}

# Output the created roles
output "created_roles" {
  description = "Map of created roles with their IDs"
  value = {
    for name, role in berth_role.teams : name => {
      id          = role.id
      description = role.description
      permissions_count = length(role.permissions)
    }
  }
}
