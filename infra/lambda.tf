# ──────────────────────────────────────────────────────────────
# Ephemeral Movie Sharing System — Lambda + API Integrations
# ──────────────────────────────────────────────────────────────

# ── Lambda Function ──────────────────────────────────────────
resource "aws_lambda_function" "movie_controller" {
  function_name = "cinebucket-controller"
  description   = "Go Lambda: Presigned URL generation and emergency bucket nuke"

  role     = aws_iam_role.lambda_exec.arn
  handler  = "bootstrap"
  runtime  = "provided.al2023"
  filename = var.lambda_zip_path

  # Recompute deployment hash from the zip contents so Terraform
  # detects new builds automatically.
  source_code_hash = filebase64sha256(var.lambda_zip_path)

  timeout     = 30
  memory_size = 128

  architectures = ["x86_64"]

  environment {
    variables = {
      S3_BUCKET_NAME         = aws_s3_bucket.ephemeral_storage.bucket
      SYSTEM_SECRET          = var.system_secret
      PRESIGN_EXPIRY_SECONDS = tostring(var.presign_expiry_seconds)
      TEARDOWN_DELAY_SECONDS = tostring(var.teardown_delay_seconds)
    }
  }

  tags = {
    Name = "MovieControllerLambda"
  }
}

# ── CloudWatch Log Group for Lambda ──────────────────────────
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${aws_lambda_function.movie_controller.function_name}"
  retention_in_days = 7

  tags = {
    Name = "LambdaExecutionLogs"
  }
}

# ── API Gateway → Lambda Integration ─────────────────────────
resource "aws_apigatewayv2_integration" "lambda_integration" {
  api_id                 = aws_apigatewayv2_api.movie_api.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.movie_controller.invoke_arn
  integration_method     = "POST"
  payload_format_version = "2.0"
}

# ── Route: POST /request-url ─────────────────────────────────
# Triggers the Lambda to generate a time-limited S3 Presigned URL.
resource "aws_apigatewayv2_route" "request_url" {
  api_id    = aws_apigatewayv2_api.movie_api.id
  route_key = "POST /request-url"
  target    = "integrations/${aws_apigatewayv2_integration.lambda_integration.id}"
}

# ── Route: POST /nuke ────────────────────────────────────────
# Emergency teardown endpoint — purges all objects from the
# ephemeral S3 bucket. Requires X-System-Secret header (validated
# inside the Go Lambda handler, not at the gateway level).
resource "aws_apigatewayv2_route" "nuke" {
  api_id    = aws_apigatewayv2_api.movie_api.id
  route_key = "POST /nuke"
  target    = "integrations/${aws_apigatewayv2_integration.lambda_integration.id}"
}

# ── Lambda Permission for API Gateway ────────────────────────
# Allow the HTTP API to invoke the Lambda function.
resource "aws_lambda_permission" "apigw_invoke" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.movie_controller.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.movie_api.execution_arn}/*/*"
}
