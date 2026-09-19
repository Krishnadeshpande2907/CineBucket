package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Movie represents a single movie entry in the catalog.
type Movie struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
}

// supportedExtensions lists the video file extensions to scan for.
var supportedExtensions = map[string]bool{
	".mp4": true,
	".mkv": true,
	".avi": true,
}

// scanDirectory walks the given directory (non-recursively) and returns
// a slice of Movie structs for every file matching a supported extension.
func scanDirectory(dir string) ([]Movie, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var movies []Movie
	counter := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !supportedExtensions[ext] {
			continue
		}
		counter++
		movies = append(movies, Movie{
			ID:       fmt.Sprintf("mov-%03d", counter),
			Title:    deriveTitle(entry.Name()),
			Filename: entry.Name(),
		})
	}

	return movies, nil
}

var yearPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)
var qualityPattern = regexp.MustCompile(`(?i)\b(1080p|720p|480p|2160p|4k|bluray|brrip|webrip|web-dl|hdtv|dvdrip|x264|x265|h264|h265|aac|dts|10bit)\b`)

// deriveTitle extracts a human-readable title from a video filename.
func deriveTitle(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(name)
	name = qualityPattern.ReplaceAllString(name, "")
	if loc := yearPattern.FindStringIndex(name); loc != nil {
		name = name[:loc[0]]
	}
	name = strings.NewReplacer("(", "", ")", "", "[", "", "]", "").Replace(name)
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	return name
}

// writeMoviesJSON marshals the movie slice to JSON and writes it to outputPath.
func writeMoviesJSON(movies []Movie, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	data, err := json.MarshalIndent(movies, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling JSON: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(outputPath, data, 0o644)
}

// movieFilePath returns the full path to a movie file within the scan directory.
func movieFilePath(dir, filename string) string {
	return filepath.Join(dir, filename)
}
