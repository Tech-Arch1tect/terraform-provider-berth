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

variable "all_server_ids" {
  description = "List of all server IDs"
  type        = list(number)
  default     = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13]
}

# Example 1: Apply same permissions to all servers
resource "berth_role" "developers" {
  name        = "developers"
  description = "Development team with access to dev stacks"

  permission_set {
    server_ids = var.all_server_ids

    permissions {
      name    = "stacks.read"
      pattern = "dev-*"
    }

    permissions {
      name    = "stacks.manage"
      pattern = "dev-*"
    }

    permissions {
      name    = "files.write"
      pattern = "dev-*"
    }

    permissions {
      name    = "logs.read"
      pattern = "dev-*"
    }
  }
}

# Example 2: Different permissions for different server groups
resource "berth_role" "operations" {
  name        = "operations"
  description = "Operations team"

  # Full access to production servers
  permission_set {
    server_ids = [1, 2, 3]  # Production servers

    permissions {
      name    = "stacks.read"
      pattern = "*"
    }

    permissions {
      name    = "stacks.manage"
      pattern = "*"
    }

    permissions {
      name    = "files.write"
      pattern = "*"
    }

    permissions {
      name    = "logs.read"
      pattern = "*"
    }
  }

  # Read-only access to staging servers
  permission_set {
    server_ids = [4, 5, 6]  # Staging servers

    permissions {
      name    = "stacks.read"
      pattern = "*"
    }

    permissions {
      name    = "logs.read"
      pattern = "*"
    }
  }
}

# Example 3: Mix permission sets with inline permissions
resource "berth_role" "qa_team" {
  name        = "qa-team"
  description = "QA team"

  # Apply to most servers via permission set
  permission_set {
    server_ids = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]

    permissions {
      name    = "stacks.read"
      pattern = "staging-*"
    }

    permissions {
      name    = "logs.read"
      pattern = "staging-*"
    }
  }

  # Special permission for one specific server
  permissions {
    server_id       = 13
    permission_name = "stacks.manage"
    stack_pattern   = "qa-test"
  }
}

# Example 4: Multiple permission sets per role
resource "berth_role" "readonly" {
  name        = "readonly"
  description = "Read-only access"

  # Read stack info on all servers
  permission_set {
    server_ids = var.all_server_ids

    permissions {
      name    = "stacks.read"
      pattern = "*"
    }
  }

  # Read logs on all servers
  permission_set {
    server_ids = var.all_server_ids

    permissions {
      name    = "logs.read"
      pattern = "*"
    }
  }
}

# Output role IDs for reference
output "role_ids" {
  value = {
    developers = berth_role.developers.id
    operations = berth_role.operations.id
    qa_team    = berth_role.qa_team.id
    readonly   = berth_role.readonly.id
  }
}
