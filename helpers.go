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

	"LNH-musictag/internal/audiofmt"
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

// fileExists reports whether a regular file exists at path.
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// stripAudioExt removes trailing audio file extensions (case-insensitive),
// repeatedly, e.g. "广岛之恋.mp3" -> "广岛之恋" and "发如雪.mp3.flac" ->
// "发如雪". Non-audio extensions or strings without one are returned unchanged.
func stripAudioExt(s string) string {
	for {
		ext := strings.ToLower(filepath.Ext(s))
		if ext != "" && (model.AudioExts[ext] || audiofmt.ExtSet[ext]) {
			s = strings.TrimSuffix(s, ext)
			continue
		}
		break
	}
	return s
}

// sanitizeName removes characters that are illegal in file names and collapses
// whitespace, e.g. "a / b" -> "a _ b".
func sanitizeName(s string) string {
	s = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// renameCleanFile renames an audio file so it keeps only its real extension,
// determined from the actual encoding (magic bytes), not the current suffix.
//   - "发如雪.mp3.flac" (real FLAC) -> "发如雪.flac"
//   - "发如雪.mp3.mp3"   (real MP3)  -> "发如雪.mp3"
//   - "song.flac" that is really MP3 -> "song.mp3"
//
// It returns the (possibly unchanged) new path. Files that are already clean,
// undetectable, or whose clean target already exists are left untouched.
func renameCleanFile(path string) (string, error) {
	base := stripAudioExt(filepath.Base(path))
	ext := audiofmt.DetectExt(path)
	if ext == "" {
		return path, nil
	}
	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, base+ext)
	if newPath == path {
		return path, nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return path, fmt.Errorf("目标已存在，跳过重命名: %s", filepath.Base(newPath))
	}
	if err := os.Rename(path, newPath); err != nil {
		return path, err
	}
	return newPath, nil
}
