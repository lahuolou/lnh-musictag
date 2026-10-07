package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"LNH-musictag/internal/model"
)

func decodeBase64Raw(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return b, nil
}

// getenvDefault returns the env value or a fallback when unset/empty.
func getenvDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fetchBytes(u string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LNH-MusicTag/0.1 (local prototype)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d fetching %s", resp.StatusCode, strings.TrimPrefix(u, "http"))
	}
	return io.ReadAll(resp.Body)
}

// stripAudioExt removes a trailing audio file extension (case-insensitive)
// from a string, e.g. "广岛之恋.mp3" -> "广岛之恋". Non-audio extensions or
// strings without one are returned unchanged.
func stripAudioExt(s string) string {
	ext := strings.ToLower(filepath.Ext(s))
	if ext != "" && model.AudioExts[ext] {
		return strings.TrimSuffix(s, ext)
	}
	return s
}
