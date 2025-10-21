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

# Define reusable server groups
locals {
  prod_servers    = [1, 2, 3]
  staging_servers = [4, 5, 6]
  dev_servers     = [7, 8, 9, 10, 11, 12, 13]
  all_servers     = concat(local.prod_servers, local.staging_servers, local.dev_servers)

  # Define reusable permission templates
  read_only_permissions = [
    { name = "stacks.read", pattern = "*" },
    { name = "logs.read", pattern = "*" }
  ]

  write_permissions = [
    { name = "stacks.manage", pattern = "*" },
    { name = "files.write", pattern = "*" }
  ]

  full_permissions = concat(local.read_only_permissions, local.write_permissions)

  # Pattern-specific permissions
  dev_stack_permissions = [
    { name = "stacks.read", pattern = "dev-*" },
    { name = "stacks.manage", pattern = "dev-*" },
    { name = "files.write", pattern = "dev-*" },
    { name = "logs.read", pattern = "dev-*" }
  ]

  staging_stack_permissions = [
    { name = "stacks.read", pattern = "staging-*" },
    { name = "stacks.manage", pattern = "staging-*" },
    { name = "logs.read", pattern = "staging-*" }
  ]
}

# Example 1: Read-only access across different environments
# Uses dynamic blocks to expand local permission template
resource "berth_role" "global_readonly" {
  name        = "global-readonly"
  description = "Read-only access to all servers"

  permission_set {
    server_ids = local.all_servers

    dynamic "permissions" {
      for_each = local.read_only_permissions
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }
}

# Example 2: Full production access
resource "berth_role" "prod_admin" {
  name        = "prod-admin"
  description = "Full access to production servers"

  permission_set {
    server_ids = local.prod_servers

    dynamic "permissions" {
      for_each = local.full_permissions
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }
}

# Example 3: Environment-specific roles reusing permission templates
resource "berth_role" "staging_operator" {
  name        = "staging-operator"
  description = "Full access to staging servers"

  permission_set {
    server_ids = local.staging_servers

    dynamic "permissions" {
      for_each = local.full_permissions  # Reuse same permissions as prod!
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }
}

# Example 4: Pattern-based permissions (only dev-* stacks)
resource "berth_role" "developer" {
  name        = "developer"
  description = "Access to dev stacks on dev servers"

  permission_set {
    server_ids = local.dev_servers

    dynamic "permissions" {
      for_each = local.dev_stack_permissions
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }
}

# Example 5: Multiple permission sets per role
resource "berth_role" "qa_team" {
  name        = "qa-team"
  description = "QA team with staging access and dev read access"

  # Full access to staging stacks
  permission_set {
    server_ids = local.staging_servers

    dynamic "permissions" {
      for_each = local.staging_stack_permissions
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }

  # Read-only access to dev servers (inline definition)
  permission_set {
    server_ids = local.dev_servers

    permissions {
      name    = "stacks.read"
      pattern = "dev-*"
    }

    permissions {
      name    = "logs.read"
      pattern = "dev-*"
    }
  }
}

# Example 6: Mix permission sets with inline permissions for exceptions
resource "berth_role" "special_operator" {
  name        = "special-operator"
  description = "Standard access plus one special server permission"

  # Standard read access to staging using template
  permission_set {
    server_ids = local.staging_servers

    dynamic "permissions" {
      for_each = local.read_only_permissions
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }

  # Special write permission on one specific server (inline permission block)
  permissions {
    server_id       = 13
    permission_name = "stacks.manage"
    stack_pattern   = "special-app"
  }
}

# Example 7: Using for_each to create similar roles for different teams
variable "team_configs" {
  description = "Team-specific role configurations"
  type = map(object({
    description = string
    server_ids  = list(number)
  }))

  default = {
    team_a = {
      description = "Team A - Servers 1-4"
      server_ids  = [1, 2, 3, 4]
    }
    team_b = {
      description = "Team B - Servers 5-8"
      server_ids  = [5, 6, 7, 8]
    }
    team_c = {
      description = "Team C - Servers 9-13"
      server_ids  = [9, 10, 11, 12, 13]
    }
  }
}

resource "berth_role" "team_roles" {
  for_each = var.team_configs

  name        = "team-${each.key}"
  description = each.value.description

  permission_set {
    server_ids = each.value.server_ids

    dynamic "permissions" {
      for_each = local.full_permissions  # All teams get same permissions!
      content {
        name    = permissions.value.name
        pattern = permissions.value.pattern
      }
    }
  }
}

# Output created role IDs
output "role_ids" {
  description = "Created role IDs"
  value = {
    global_readonly   = berth_role.global_readonly.id
    prod_admin        = berth_role.prod_admin.id
    staging_operator  = berth_role.staging_operator.id
    developer         = berth_role.developer.id
    qa_team           = berth_role.qa_team.id
    special_operator  = berth_role.special_operator.id
    team_roles        = { for k, v in berth_role.team_roles : k => v.id }
  }
}
