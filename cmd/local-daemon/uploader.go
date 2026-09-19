package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// progressReader wraps an io.Reader to track upload progress.
type progressReader struct {
	reader io.Reader
	total  int64
	read   int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)
	if pr.total > 0 {
		pct := float64(pr.read) / float64(pr.total) * 100
		fmt.Printf("\r⏳ Uploading: %.1f%% (%d/%d bytes)", pct, pr.read, pr.total)
	}
	return n, err
}

// uploadFile uploads a local file to S3 using multipart upload manager.
func uploadFile(ctx context.Context, s3Client *s3.Client, bucket, key, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening local file %s: %w", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file %s: %w", filePath, err)
	}

	pr := &progressReader{
		reader: file,
		total:  stat.Size(),
	}

	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // 10 MB parts
		u.Concurrency = 3
	})

	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   pr,
	})
	fmt.Println() // Newline after progress printing completes
	if err != nil {
		return fmt.Errorf("s3 upload failed: %w", err)
	}

	return nil
}

// nukeResult represents the JSON response from POST /nuke.
type nukeResult struct {
	Status              string `json:"status"`
	DeletedObjectsCount int    `json:"deleted_objects_count"`
}

// sendNukeRequest sends an HTTP POST request to the API Gateway /nuke endpoint.
func sendNukeRequest(apiGatewayURL, systemSecret string) (*nukeResult, error) {
	url := fmt.Sprintf("%s/nuke", apiGatewayURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-System-Secret", systemSecret)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	var res nukeResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &res, nil
}

// stdinScanner provides context-aware stdin line reading.
type stdinScanner struct {
	lines chan string
}

func newStdinScanner() *stdinScanner {
	s := &stdinScanner{
		lines: make(chan string),
	}
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			s.lines <- scanner.Text()
		}
		close(s.lines)
	}()
	return s
}

func (s *stdinScanner) readLine(ctx context.Context) (string, bool) {
	select {
	case <-ctx.Done():
		return "", false
	case line, ok := <-s.lines:
		return line, ok
	}
}
