# ──────────────────────────────────────────────────────────────
# Ephemeral Movie Sharing System — Configurable Variables
# ──────────────────────────────────────────────────────────────

variable "aws_region" {
  description = "AWS region for all resources."
  type        = string
  default     = "ap-south-1"
}

variable "environment" {
  description = "Deployment environment tag (e.g. dev, staging, prod)."
  type        = string
  default     = "dev"
}

variable "domain_name" {
  description = "Custom domain for the frontend (e.g. mov.example.in)."
  type        = string
  default     = "mov.example.in"
}

variable "s3_bucket_name" {
  description = "Name of the ephemeral S3 bucket for movie uploads."
  type        = string
  default     = "cinebucket-bucket"
}

variable "lambda_zip_path" {
  description = "Path to the compiled Lambda deployment package (zip)."
  type        = string
  default     = "../cmd/lambda-controller/bootstrap.zip"
}

variable "system_secret" {
  description = "Shared secret for authenticating /nuke requests via X-System-Secret header."
  type        = string
  sensitive   = true
}

variable "presign_expiry_seconds" {
  description = "Lifetime in seconds for generated S3 Presigned URLs."
  type        = number
  default     = 600
}

variable "teardown_delay_seconds" {
  description = "Delay in seconds after approval before the cleanup routine fires."
  type        = number
  default     = 720
}

variable "default_tags" {
  description = "Default resource tags applied to every AWS resource."
  type        = map(string)
  default = {
    Project   = "CineBucket"
    ManagedBy = "Terraform"
  }
}
