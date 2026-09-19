// Package main implements the cinebucket local daemon — a Go CLI tool
// that manages local movie catalogues, uploads to ephemeral S3 storage,
// and triggers remote teardown via the CineBucket API.
//
// Usage:
//
//	cinebucket sync  --dir=/path/to/movies   # Scan, build movies.json, git push
//	cinebucket serve --dir=/path/to/movies   # Interactive upload daemon
//	cinebucket nuke                           # Emergency remote bucket purge
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// appConfig holds runtime configuration loaded from environment variables.
type appConfig struct {
	APIGatewayURL string // Base URL of the API Gateway (e.g. https://<id>.execute-api.<region>.amazonaws.com)
	SystemSecret  string // Shared secret for /nuke authentication
	S3BucketName  string // Target ephemeral S3 bucket
}

// loadConfig reads required environment variables and returns a validated config.
func loadConfig() (*appConfig, error) {
	cfg := &appConfig{
		APIGatewayURL: os.Getenv("API_GATEWAY_URL"),
		SystemSecret:  os.Getenv("SYSTEM_SECRET"),
		S3BucketName:  os.Getenv("S3_BUCKET_NAME"),
	}

	// Trim trailing slash from API URL for consistent path joining.
	cfg.APIGatewayURL = strings.TrimRight(cfg.APIGatewayURL, "/")

	var missing []string
	if cfg.APIGatewayURL == "" {
		missing = append(missing, "API_GATEWAY_URL")
	}
	if cfg.SystemSecret == "" {
		missing = append(missing, "SYSTEM_SECRET")
	}
	if cfg.S3BucketName == "" {
		missing = append(missing, "S3_BUCKET_NAME")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `CineBucket CLI (cinebucket)

Usage:
  cinebucket <command> [flags]

Commands:
  sync   --dir=<path>    Scan directory for movies, update web/movies.json, and git push.
  serve  --dir=<path>    Run interactive upload daemon (awaits user approval to push files to S3).
  nuke                   Emergency teardown — purge all objects from the remote S3 bucket.

Environment Variables (required):
  API_GATEWAY_URL    Base URL of the CineBucket API Gateway.
  SYSTEM_SECRET      Shared secret for authenticating /nuke requests.
  S3_BUCKET_NAME     Name of the target ephemeral S3 bucket.

Examples:
  cinebucket sync  --dir=./movies
  cinebucket serve --dir=/home/user/media/movies
  cinebucket nuke
`)
}

// parseDirFlag extracts --dir=<value> from os.Args[2:].
func parseDirFlag() (string, error) {
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--dir=") {
			dir := strings.TrimPrefix(arg, "--dir=")
			if dir == "" {
				return "", fmt.Errorf("--dir value cannot be empty")
			}
			return dir, nil
		}
	}
	return "", fmt.Errorf("--dir flag is required")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Create a root context that cancels on SIGINT / SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	command := strings.ToLower(os.Args[1])

	switch command {
	case "sync":
		runSync()

	case "serve":
		runServe(ctx)

	case "nuke":
		runNuke()

	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// ── sync ────────────────────────────────────────────────────────────────────
// Scans a local directory for movie files, writes web/movies.json,
// then git add → commit → push.
func runSync() {
	dir, err := parseDirFlag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔍 Scanning directory: %s\n", dir)

	movies, err := scanDirectory(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	if len(movies) == 0 {
		fmt.Println("⚠️  No movie files found (.mp4, .mkv, .avi)")
		os.Exit(0)
	}

	fmt.Printf("📽  Found %d movie(s):\n", len(movies))
	for _, m := range movies {
		fmt.Printf("   • [%s] %s (%s)\n", m.ID, m.Title, m.Filename)
	}

	outputPath := "web/movies.json"
	if err := writeMoviesJSON(movies, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
		os.Exit(1)
	}
	fmt.Printf("✅ Written %s (%d entries)\n", outputPath, len(movies))

	// Git operations: add → commit → push.
	fmt.Println("📦 Committing to Git...")

	if err := gitAdd(outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: git add failed: %v\n", err)
		os.Exit(1)
	}

	if err := gitCommit("chore: sync catalog"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: git commit failed: %v\n", err)
		os.Exit(1)
	}

	if err := gitPush("origin", "main"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: git push failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Catalog synced and pushed to origin/main")
}

// ── serve ───────────────────────────────────────────────────────────────────
// Runs an interactive upload daemon. Lists available movies, prompts the
// user for approval, and uploads selected files to S3 using the multipart
// upload manager. Handles graceful shutdown via context cancellation.
func runServe(ctx context.Context) {
	dir, err := parseDirFlag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialise AWS SDK client.
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading AWS config: %v\n", err)
		os.Exit(1)
	}
	s3Client := s3.NewFromConfig(awsCfg)

	fmt.Println("────────────────────────────────────────────")
	fmt.Println("  CineBucket — Upload Daemon")
	fmt.Printf("  Bucket : %s\n", cfg.S3BucketName)
	fmt.Printf("  Dir    : %s\n", dir)
	fmt.Println("────────────────────────────────────────────")
	fmt.Println("Type a movie number to upload, 'list' to refresh, or 'quit' to exit.")
	fmt.Println()

	// Main interactive loop.
	movies, err := scanDirectory(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}
	printMovieList(movies)

	scanner := newStdinScanner()
	for {
		fmt.Print("\n▶ Enter movie number (or 'list' / 'quit'): ")

		select {
		case <-ctx.Done():
			fmt.Println("\n🛑 Shutting down gracefully...")
			return
		default:
		}

		input, ok := scanner.readLine(ctx)
		if !ok {
			fmt.Println("\n🛑 Shutting down...")
			return
		}

		input = strings.TrimSpace(input)

		switch strings.ToLower(input) {
		case "quit", "exit", "q":
			fmt.Println("👋 Exiting upload daemon.")
			return
		case "list", "ls", "l":
			movies, err = scanDirectory(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error rescanning: %v\n", err)
				continue
			}
			printMovieList(movies)
			continue
		case "":
			continue
		}

		// Parse the movie number.
		var idx int
		if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(movies) {
			fmt.Printf("⚠️  Invalid selection. Enter a number between 1 and %d.\n", len(movies))
			continue
		}

		movie := movies[idx-1]
		filePath := movieFilePath(dir, movie.Filename)

		fmt.Printf("⬆️  Uploading %s → s3://%s/%s ...\n", movie.Filename, cfg.S3BucketName, movie.Filename)

		if err := uploadFile(ctx, s3Client, cfg.S3BucketName, movie.Filename, filePath); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Upload failed: %v\n", err)
			continue
		}

		fmt.Printf("✅ Upload complete: %s\n", movie.Filename)
	}
}

// printMovieList displays a numbered list of available movies.
func printMovieList(movies []Movie) {
	if len(movies) == 0 {
		fmt.Println("  (no movie files found)")
		return
	}
	fmt.Printf("  Available movies (%d):\n", len(movies))
	for i, m := range movies {
		fmt.Printf("    %2d. %s  (%s)\n", i+1, m.Title, m.Filename)
	}
}

// ── nuke ────────────────────────────────────────────────────────────────────
// Issues an HTTP POST to the API Gateway /nuke endpoint with the
// X-System-Secret header. Prints the result to stdout.
func runNuke() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("💣 Sending nuke request to %s/nuke ...\n", cfg.APIGatewayURL)

	result, err := sendNukeRequest(cfg.APIGatewayURL, cfg.SystemSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Nuke failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Nuke complete — status=%s, deleted_objects_count=%d\n",
		result.Status, result.DeletedObjectsCount)
}
