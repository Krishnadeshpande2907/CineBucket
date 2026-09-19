package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// presignRequest is the expected JSON body for POST /request-url.
type presignRequest struct {
	Filename string `json:"filename"`
}

// presignResponse is the JSON payload returned on a successful presign.
type presignResponse struct {
	Status             string `json:"status"`
	DownloadURL        string `json:"download_url"`
	ExpiresInSeconds   int    `json:"expires_in_seconds"`
	TeardownAtSeconds  int    `json:"teardown_at_seconds"`
}

// handlePresignRequest validates the incoming filename, generates a
// time-limited S3 GetObject Presigned URL, and returns the download link.
func (h *handler) handlePresignRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// ── Parse request body ──────────────────────────────────
	var body presignRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		log.Printf("ERROR: failed to parse request body: %v", err)
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
	}

	body.Filename = strings.TrimSpace(body.Filename)
	if body.Filename == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "filename is required",
		})
	}

	// ── Validate filename (basic path traversal guard) ──────
	if strings.Contains(body.Filename, "..") || strings.ContainsAny(body.Filename, "/\\") {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "filename must not contain path separators or '..'",
		})
	}

	log.Printf("Generating presigned URL: bucket=%s key=%s expiry=%ds",
		h.cfg.BucketName, body.Filename, h.cfg.PresignExpirySecs)

	// ── Generate S3 GetObject Presigned URL ──────────────────
	presignedReq, err := h.clients.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &h.cfg.BucketName,
		Key:    &body.Filename,
	}, s3.WithPresignExpires(time.Duration(h.cfg.PresignExpirySecs)*time.Second))
	if err != nil {
		log.Printf("ERROR: presign failed: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to generate presigned URL: %v", err),
		})
	}

	log.Printf("Presigned URL generated successfully for %s (expires in %ds)", body.Filename, h.cfg.PresignExpirySecs)

	// ── Return success payload ──────────────────────────────
	return jsonResponse(http.StatusOK, presignResponse{
		Status:            "approved",
		DownloadURL:       presignedReq.URL,
		ExpiresInSeconds:  h.cfg.PresignExpirySecs,
		TeardownAtSeconds: h.cfg.TeardownDelaySecs,
	})
}
