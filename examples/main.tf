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
  # insecure_skip_verify = true  # Uncomment if using self-signed certificates
}

# ============================================================================
# OPTION 1: Inline Permissions (Recommended for most use cases)
# ============================================================================

# Create a role for developers with inline permissions
resource "berth_role" "developers" {
  name        = "developers"
  description = "Development team with access to dev stacks"

  permissions {
    server_id       = 1
    permission_name = "stacks.read"
    stack_pattern   = "dev-*"
  }

  permissions {
    server_id       = 1
    permission_name = "stacks.manage"
    stack_pattern   = "dev-*"
  }

  permissions {
    server_id       = 1
    permission_name = "files.write"
    stack_pattern   = "dev-*"
  }

  permissions {
    server_id       = 1
    permission_name = "logs.read"
    stack_pattern   = "dev-*"
  }
}

# Create a role for operations team with inline permissions
resource "berth_role" "operations" {
  name        = "operations"
  description = "Operations team with broader access"

  permissions {
    server_id       = 1
    permission_name = "stacks.read"
    stack_pattern   = "*"
  }

  permissions {
    server_id       = 1
    permission_name = "stacks.manage"
    stack_pattern   = "*"
  }

  permissions {
    server_id       = 1
    permission_name = "files.write"
    stack_pattern   = "*"
  }
}

# ============================================================================
# OPTION 2: Separate Permission Resources (For advanced use cases)
# ============================================================================

# Create a role without inline permissions
resource "berth_role" "qa_team" {
  name        = "qa-team"
  description = "QA team"
}

# Add permissions to QA team separately (useful for dynamic permission management)
resource "berth_role_permission" "qa_read_staging" {
  role_id         = berth_role.qa_team.id
  server_id       = 1
  permission_name = "stacks.read"
  stack_pattern   = "staging-*"
}

resource "berth_role_permission" "qa_logs_staging" {
  role_id         = berth_role.qa_team.id
  server_id       = 1
  permission_name = "logs.read"
  stack_pattern   = "staging-*"
}
