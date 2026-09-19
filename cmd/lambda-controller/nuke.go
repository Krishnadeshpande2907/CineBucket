package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// nukeResponse is the JSON payload returned after a successful bucket purge.
type nukeResponse struct {
	Status              string `json:"status"`
	DeletedObjectsCount int    `json:"deleted_objects_count"`
}

// handleNukeRequest validates the X-System-Secret header, then iteratively
// lists and batch-deletes every object in the ephemeral S3 bucket until empty.
func (h *handler) handleNukeRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// ── Authenticate via X-System-Secret header ─────────────
	// API Gateway v2 lowercases all header keys.
	secret := req.Headers["x-system-secret"]
	if strings.TrimSpace(secret) == "" {
		log.Println("WARN: nuke request missing X-System-Secret header")
		return jsonResponse(http.StatusUnauthorized, map[string]string{
			"error": "missing X-System-Secret header",
		})
	}
	if secret != h.cfg.SystemSecret {
		log.Println("WARN: nuke request with invalid X-System-Secret")
		return jsonResponse(http.StatusForbidden, map[string]string{
			"error": "invalid system secret",
		})
	}

	log.Printf("Nuke authorised — purging all objects from bucket: %s", h.cfg.BucketName)

	// ── Iteratively list and delete all objects ──────────────
	totalDeleted := 0

	for {
		listOutput, err := h.clients.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: &h.cfg.BucketName,
		})
		if err != nil {
			log.Printf("ERROR: ListObjectsV2 failed: %v", err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to list bucket objects: %v", err),
			})
		}

		if len(listOutput.Contents) == 0 {
			break
		}

		// Build the batch delete identifiers.
		objectIDs := make([]types.ObjectIdentifier, 0, len(listOutput.Contents))
		for _, obj := range listOutput.Contents {
			objectIDs = append(objectIDs, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		deleteOutput, err := h.clients.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &h.cfg.BucketName,
			Delete: &types.Delete{
				Objects: objectIDs,
				Quiet:   boolPtr(true),
			},
		})
		if err != nil {
			log.Printf("ERROR: DeleteObjects failed: %v", err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to delete objects: %v", err),
			})
		}

		// Count errors — S3 batch delete may partially fail.
		if len(deleteOutput.Errors) > 0 {
			for _, e := range deleteOutput.Errors {
				log.Printf("ERROR: failed to delete key=%s: code=%s message=%s",
					ptrStr(e.Key), ptrStr(e.Code), ptrStr(e.Message))
			}
			return jsonResponse(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("%d objects failed to delete", len(deleteOutput.Errors)),
			})
		}

		batchCount := len(objectIDs)
		totalDeleted += batchCount
		log.Printf("Deleted batch of %d objects (running total: %d)", batchCount, totalDeleted)

		// If the listing was not truncated, we've cleared everything.
		if !*listOutput.IsTruncated {
			break
		}
	}

	log.Printf("Nuke complete — %d objects destroyed from %s", totalDeleted, h.cfg.BucketName)

	return jsonResponse(http.StatusOK, nukeResponse{
		Status:              "nuked",
		DeletedObjectsCount: totalDeleted,
	})
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool { return &v }

// ptrStr safely dereferences a *string, returning "" if nil.
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
