# ──────────────────────────────────────────────────────────────
# Ephemeral Movie Sharing System — Core Infrastructure
# Provider · S3 Ephemeral Bucket · API Gateway v2 HTTP API
# ──────────────────────────────────────────────────────────────

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# ── AWS Provider ─────────────────────────────────────────────
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = var.default_tags
  }
}

# ── S3 Ephemeral Storage Bucket ──────────────────────────────
resource "aws_s3_bucket" "ephemeral_storage" {
  bucket        = var.s3_bucket_name
  force_destroy = true # Allow Terraform to destroy non-empty buckets during teardown

  tags = {
    Name = "CineBucketStorage"
  }
}

# Block ALL public access — downloads are served exclusively
# through time-limited S3 Presigned URLs.
resource "aws_s3_bucket_public_access_block" "ephemeral_privacy" {
  bucket                  = aws_s3_bucket.ephemeral_storage.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Enable server-side encryption at rest (AES-256 / SSE-S3).
resource "aws_s3_bucket_server_side_encryption_configuration" "ephemeral_sse" {
  bucket = aws_s3_bucket.ephemeral_storage.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Disable bucket versioning — objects are ephemeral by design.
resource "aws_s3_bucket_versioning" "ephemeral_versioning" {
  bucket = aws_s3_bucket.ephemeral_storage.id

  versioning_configuration {
    status = "Disabled"
  }
}

# ── API Gateway v2 (HTTP API) ────────────────────────────────
resource "aws_apigatewayv2_api" "movie_api" {
  name          = "cinebucket-api"
  protocol_type = "HTTP"
  description   = "HTTP API for the CineBucket System"

  cors_configuration {
    allow_origins = [
      "https://${var.domain_name}",
      "http://localhost:*"
    ]
    allow_methods = ["POST", "OPTIONS"]
    allow_headers = ["Content-Type", "X-System-Secret"]
    max_age       = 3600
  }

  tags = {
    Name = "CineBucketAPI"
  }
}

# Auto-deploy stage — every integration change goes live immediately.
resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.movie_api.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gw_logs.arn
    format = jsonencode({
      requestId        = "$context.requestId"
      ip               = "$context.identity.sourceIp"
      requestTime      = "$context.requestTime"
      httpMethod       = "$context.httpMethod"
      routeKey         = "$context.routeKey"
      status           = "$context.status"
      protocol         = "$context.protocol"
      responseLength   = "$context.responseLength"
      integrationError = "$context.integrationErrorMessage"
    })
  }

  tags = {
    Name = "DefaultStage"
  }
}

# CloudWatch Log Group for API Gateway access logs.
resource "aws_cloudwatch_log_group" "api_gw_logs" {
  name              = "/aws/apigateway/cinebucket-api"
  retention_in_days = 7

  tags = {
    Name = "APIGatewayAccessLogs"
  }
}
