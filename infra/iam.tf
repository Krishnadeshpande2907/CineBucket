# ──────────────────────────────────────────────────────────────
# Ephemeral Movie Sharing System — IAM (Least Privilege)
# ──────────────────────────────────────────────────────────────

# ── Lambda Execution Role ────────────────────────────────────
resource "aws_iam_role" "lambda_exec" {
  name = "cinebucket-lambda-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Name = "LambdaExecRole"
  }
}

# ── S3 Scoped Policy (Least Privilege) ───────────────────────
# Lambda may ONLY perform GetObject, PutObject, DeleteObject on
# objects in the ephemeral bucket, and ListBucket on the bucket
# itself. No wildcard s3:* permissions.
resource "aws_iam_role_policy" "lambda_s3_access" {
  name = "cinebucket-s3-scoped"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowObjectOps"
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = "${aws_s3_bucket.ephemeral_storage.arn}/*"
      },
      {
        Sid    = "AllowBucketList"
        Effect = "Allow"
        Action = [
          "s3:ListBucket"
        ]
        Resource = aws_s3_bucket.ephemeral_storage.arn
      }
    ]
  })
}

# ── CloudWatch Logs Policy (Basic Lambda Execution) ──────────
# Attach the AWS-managed policy so Lambda can write logs to
# CloudWatch. This is the minimum required for observability.
resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}
