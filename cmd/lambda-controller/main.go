// Package main is the Lambda entrypoint for the CineBucket controller.
// It routes incoming API Gateway v2 HTTP API requests to the appropriate
// handler based on the route key (POST /request-url, POST /nuke).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// cfg holds runtime configuration loaded from environment variables.
type cfg struct {
	BucketName         string
	SystemSecret       string
	PresignExpirySecs  int
	TeardownDelaySecs  int
}

// clients holds the initialised AWS SDK service clients.
type clients struct {
	s3Client      *s3.Client
	presignClient *s3.PresignClient
}

// handler wraps configuration and clients so they survive across warm starts.
type handler struct {
	cfg     cfg
	clients clients
}

// newHandler reads environment variables and initialises AWS SDK clients.
func newHandler(ctx context.Context) (*handler, error) {
	bucketName := os.Getenv("S3_BUCKET_NAME")
	if bucketName == "" {
		return nil, fmt.Errorf("S3_BUCKET_NAME environment variable is required")
	}

	systemSecret := os.Getenv("SYSTEM_SECRET")
	if systemSecret == "" {
		return nil, fmt.Errorf("SYSTEM_SECRET environment variable is required")
	}

	presignExpiry := 600 // default 10 minutes
	if v := os.Getenv("PRESIGN_EXPIRY_SECONDS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PRESIGN_EXPIRY_SECONDS: %w", err)
		}
		presignExpiry = parsed
	}

	teardownDelay := 720 // default 12 minutes
	if v := os.Getenv("TEARDOWN_DELAY_SECONDS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid TEARDOWN_DELAY_SECONDS: %w", err)
		}
		teardownDelay = parsed
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	return &handler{
		cfg: cfg{
			BucketName:        bucketName,
			SystemSecret:      systemSecret,
			PresignExpirySecs: presignExpiry,
			TeardownDelaySecs: teardownDelay,
		},
		clients: clients{
			s3Client:      s3Client,
			presignClient: s3.NewPresignClient(s3Client),
		},
	}, nil
}

// route dispatches the request to the correct handler based on route key.
func (h *handler) route(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("Received request: method=%s path=%s routeKey=%s", req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path, req.RouteKey)

	switch req.RouteKey {
	case "POST /request-url":
		return h.handlePresignRequest(ctx, req)
	case "POST /nuke":
		return h.handleNukeRequest(ctx, req)
	default:
		return jsonResponse(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("unknown route: %s", req.RouteKey),
		})
	}
}

// jsonResponse is a helper that marshals a payload into an API Gateway response.
func jsonResponse(statusCode int, body any) (events.APIGatewayV2HTTPResponse, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error":"failed to marshal response"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func main() {
	ctx := context.Background()

	h, err := newHandler(ctx)
	if err != nil {
		log.Fatalf("Failed to initialise handler: %v", err)
	}

	log.Printf("CineBucket Lambda controller initialised (bucket=%s, presign_expiry=%ds, teardown_delay=%ds)",
		h.cfg.BucketName, h.cfg.PresignExpirySecs, h.cfg.TeardownDelaySecs)

	lambda.Start(h.route)
}
