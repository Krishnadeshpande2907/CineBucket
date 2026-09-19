# CineBucket — Ephemeral Movie Sharing System

> **cinebucket.in** — On-demand, self-destroying movie catalogue with zero-persist cloud infrastructure.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Terraform](https://img.shields.io/badge/Terraform-1.5%2B-7b4288?logo=terraform)
![Go](https://img.shields.io/badge/Go-1.21%2B-00add8?logo=go)
![AWS](https://img.shields.io/badge/AWS-S3--Lambda--APIGW-232f3e?logo=amazon-aws)

---

## Overview

CineBucket is an event-driven, ephemeral file distribution platform. A local Go CLI daemon manages your movie catalogue and uploads files to AWS on user approval. The web frontend displays a searchable movie catalogue and provides download links via time-limited S3 Presigned URLs. All cloud resources self-destruct automatically, ensuring **zero permanent storage**.

### Architecture

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  GitHub Pages │     │  AWS API Gateway │     │  AWS Lambda          │
│  (cinebucket  │◄───►│  (HTTP API)      │◄───►│  (Go: presigned URL │
│   .in)        │     └──────────────────┘     │   & bucket nuke)     │
└──────┬───────┘                               └─────────┬───────────┘
       │                                                  │
       │  POST /request-url    POST /nuke                 │ S3
       │         │                    │                    │
       │         └────────┬───────────┘                    ▼
       │                  │                    ┌──────────────────┐
       │                  ▼                    │  Ephemeral S3    │
       │         ┌──────────────────┐          │  (AES-256, blocked│
       │         │  Local Go Daemon │          │   from public)   │
       │         │  (cinebucket     │          └──────────────────┘
       │         │   serve)         │
       │         └───────┬──────────┘
       │                 │  Upload via multipart
       │                 ▼
       │         ┌──────────────────┐
       └─────────│  Local Movies    │
         (12-min │  Directory       │
          buffer)└──────────────────┘
```

### Key Principles

- **Zero-Persist**: No permanent video files in AWS. Everything self-destructs.
- **Link Expiry**: S3 Presigned URLs expire after **10 minutes**.
- **Auto Teardown**: Cloud resources purged at **12 minutes** post-approval (2-minute download buffer).
- **Zero-Trust**: S3 bucket blocks all public access; downloads exclusively via presigned URLs.
- **Least Privilege**: Lambda IAM role restricted to `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject`, `s3:ListBucket`.

---

## Project Structure

```
CineBucket/
├── infra/                     # Terraform Infrastructure as Code
│   ├── main.tf                # AWS Provider, S3 Bucket, API Gateway v2
│   ├── lambda.tf              # Lambda function, API integrations
│   ├── iam.tf                 # IAM roles (least privilege)
│   ├── variables.tf           # Configurable variables
│   └── outputs.tf             # API endpoints & bucket names
├── cmd/
│   ├── lambda-controller/     # AWS Lambda (Go)
│   │   ├── main.go            # Route router & entrypoint
│   │   ├── presign.go         # S3 Presigned URL generator
│   │   └── nuke.go            # Bucket purge handler
│   └── local-daemon/          # Go CLI (cinebucket)
│       ├── main.go            # CLI entrypoint (sync, serve, nuke)
│       ├── scanner.go         # Directory scanner & movies.json builder
│       ├── uploader.go        # S3 multipart uploader
│       └── git.go             # Git exec wrapper
└── web/                       # Static Frontend (GitHub Pages)
    ├── index.html             # Movie catalogue UI
    ├── app.js                 # API integration & countdown timer
    ├── styles.css             # Dark glassmorphic design
    └── movies.json            # Movie catalogue data
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend / Lambda | Go 1.21+, `aws-sdk-go-v2`, `aws-lambda-go` |
| Local CLI | Go 1.21+, `aws-sdk-go-v2` |
| Infrastructure | Terraform 1.5+, AWS Provider ~> 5.0 |
| Frontend | Vanilla HTML5, CSS3, JavaScript (ES6+) |
| Storage | Amazon S3 (ephemeral, SSE-S3 encrypted) |
| Compute | AWS Lambda (provided.al2023 runtime) |
| API | API Gateway v2 (HTTP API) |
| Hosting | GitHub Pages (custom domain via Route 53 / ACM) |

---

## Prerequisites

- **Go** 1.21+ ([golang.org](https://golang.org))
- **Terraform** 1.5+ ([hashicorp.com](https://hashicorp.com))
- **AWS CLI** v2 configured with credentials
- **Git**
- **Node.js** (optional, only for local frontend testing)

---

## Quick Start

### 1. Deploy Cloud Infrastructure

```bash
cd infra
terraform init
terraform apply
```

After deployment, note the outputs:
- `api_gateway_url` — your API Gateway endpoint
- `s3_bucket_name` — your ephemeral S3 bucket

### 2. Build the Lambda Controller

```bash
cd cmd/lambda-controller
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap main.go
zip function.zip bootstrap
```

Terraform automatically deploys this via `lambda.tf`.

### 3. Build the Local CLI Daemon

```bash
cd cmd/local-daemon
go build -o cinebucket main.go
```

### 4. Configure Environment Variables

```bash
export API_GATEWAY_URL="https://<your-api-id>.execute-api.<region>.amazonaws.com"
export SYSTEM_SECRET="<your-shared-secret>"
export S3_BUCKET_NAME="cinebucket-bucket"
```

### 5. Sync Movie Catalogue

```bash
./cinebucket sync --dir=/path/to/movies
```

This scans the directory for `.mp4`, `.mkv`, `.avi` files, generates `web/movies.json`, and pushes to Git.

### 6. Run the Upload Daemon

```bash
./cinebucket serve --dir=/path/to/movies
```

The interactive daemon lists available movies. Enter a movie number to upload to S3 after user approval.

### 7. Emergency Nuke

```bash
./cinebucket nuke
```

Instantly purges all objects from the S3 bucket via the API Gateway `/nuke` endpoint.

---

## CLI Commands Reference

### `cinebucket sync --dir=<path>`

Scans a local directory for video files, generates `web/movies.json`, then runs `git add → commit → push`.

### `cinebucket serve --dir=<path>`

Interactive upload daemon. Lists movies, prompts for approval, and uploads selected files to S3 using multipart upload. Gracefully handles SIGINT/SIGTERM.

### `cinebucket nuke`

Emergency teardown. Sends a POST request to `/nuke` with the `X-System-Secret` header to purge all S3 objects.

### Environment Variables (Required)

| Variable | Description |
|----------|-------------|
| `API_GATEWAY_URL` | Base URL of the CineBucket API Gateway |
| `SYSTEM_SECRET` | Shared secret for `/nuke` authentication |
| `S3_BUCKET_NAME` | Target ephemeral S3 bucket name |

---

## Frontend

The web frontend lives in `web/` and is designed to be hosted on **GitHub Pages** at `cinebucket.in`.

### Local Testing

```bash
cd web
python3 -m http.server 8000
# Open http://localhost:8000
```

### Features

- **Browse Catalogue**: View all movies with posters, ratings, genres, and cast
- **Search & Filter**: Search by title, actor, or genre; filter by genre/actor dropdowns
- **Sort**: By IMDB rating, release year, alphabetical, or default order
- **Genre Pills**: Quick genre filtering via pill buttons
- **Floating Details**: Click a card for synopsis, cast, and download request
- **Countdown Timer**: Real-time 10-minute countdown with download link
- **Link Expiry Handling**: Friendly "Expired Link" modal when S3 access fails
- **API Config**: Configure API Gateway URL via settings modal

### Deployment (GitHub Pages)

1. Push `web/` contents to a GitHub repository
2. Enable GitHub Pages in repository Settings → Pages (source: `main` branch, `/root`)
3. Configure custom domain `cinebucket.in` via Route 53 + ACM SSL

---

## Deployment Workflow

| Step | Component | Action |
|------|-----------|--------|
| 1 | Terraform | `terraform init && terraform apply` |
| 2 | Lambda | Build & deploy Go binary |
| 3 | Local Daemon | Build `cinebucket`, test sync/serve/nuke |
| 4 | Frontend | Host `web/` on GitHub Pages |
| 5 | E2E | Full integration test with real user flow |

---

## Verification Checklist

| Check | Command / Action | Expected |
|-------|-----------------|----------|
| IaC Valid | `cd infra && terraform validate` | 0 errors |
| S3 Private | AWS Console → Bucket → Block Public Access = ON | All blocks enabled |
| Presign URL | `curl -X POST <api>/request-url -d '{"filename":"test.mp4"}'` | Returns `download_url` |
| Local Sync | `./cinebucket sync --dir=./test-movies` | `web/movies.json` updated & pushed |
| Upload | `./cinebucket serve` → select movie → upload | `aws s3 ls s3://<bucket>` shows file |
| Nuke | `./cinebucket nuke` | Bucket is empty |

---

## Security

- **No hardcoded credentials** — all config via environment variables
- **S3 Access Block** — all public access blocked; downloads via presigned URLs only
- **SSE-S3 Encryption** — all objects encrypted at rest (AES-256)
- **IAM Least Privilege** — Lambda scoped to specific bucket operations only
- **Secret Authentication** — `/nuke` endpoint requires valid `X-System-Secret` header
- **No Versioning** — objects are ephemeral; versioning disabled

---

## Contributing

This project is designed for AI-assisted development with clear module assignments:

- **`infra/`** — Declarative Terraform HCL
- **`cmd/lambda-controller/`** — Serverless Go backend
- **`cmd/local-daemon/`** — Go CLI tooling
- **`web/`** — Lightweight static frontend

---

## License

MIT
