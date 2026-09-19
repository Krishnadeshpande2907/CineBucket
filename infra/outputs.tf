# ──────────────────────────────────────────────────────────────
# Ephemeral Movie Sharing System — Outputs
# ──────────────────────────────────────────────────────────────

output "api_gateway_url" {
  description = "Base invoke URL for the HTTP API (used by web frontend and local CLI)."
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "api_gateway_id" {
  description = "API Gateway v2 resource ID."
  value       = aws_apigatewayv2_api.movie_api.id
}

output "s3_bucket_name" {
  description = "Name of the ephemeral S3 storage bucket."
  value       = aws_s3_bucket.ephemeral_storage.bucket
}

output "s3_bucket_arn" {
  description = "ARN of the ephemeral S3 storage bucket."
  value       = aws_s3_bucket.ephemeral_storage.arn
}

output "lambda_function_name" {
  description = "Name of the deployed Lambda controller function."
  value       = aws_lambda_function.movie_controller.function_name
}
