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

variable "s3_access_key_id" {
  description = "S3 access key ID for the backup bucket"
  type        = string
  sensitive   = true
}

variable "s3_secret_access_key" {
  description = "S3 secret access key for the backup bucket"
  type        = string
  sensitive   = true
}

resource "berth_s3_bucket" "backups" {
  label             = "primary-backups"
  endpoint          = "https://s3.eu-west-2.amazonaws.com"
  region            = "eu-west-2"
  bucket_name       = "example-berth-backups"
  access_key_id     = var.s3_access_key_id
  secret_access_key = var.s3_secret_access_key
}

output "backup_bucket_id" {
  description = "Berth ID of the backup bucket credential"
  value       = berth_s3_bucket.backups.id
}
