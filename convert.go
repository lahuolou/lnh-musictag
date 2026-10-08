// 格式转换：把选中曲目转码为 mp3/flac/m4a/ogg/opus/wav（依赖 ffmpeg），
// 转换后重新登记曲目（保留标签元数据），可选删除原文件。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"LNH-musictag/internal/dedup"
)

// convertCodecs maps target extension -> ffmpeg audio codec args.
var convertCodecs = map[string]string{
	"mp3":  "libmp3lame -q:a 2",
	"flac": "flac",
	"m4a":  "aac -b:a 192k",
	"ogg":  "libvorbis -q:a 5",
	"opus": "libopus -b:a 128k",
	"wav":  "pcm_s16le",
}

// convertHandler transcodes selected tracks to a target format.
func convertHandler(s *store, fp dedup.Fingerprinter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs          []string `json:"ids"`
			Target       string   `json:"target"`
			RemoveSource bool     `json:"removeSource"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		target := strings.ToLower(strings.TrimSpace(req.Target))
		codec, ok := convertCodecs[target]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "不支持的格式，可选: mp3/flac/m4a/ogg/opus/wav"})
			return
		}
		list := resolveTracks(s, req.IDs)
		type res struct {
			ID     string `json:"id"`
			Before string `json:"before"`
			After  string `json:"after"`
			OK     bool   `json:"ok"`
			Error  string `json:"error,omitempty"`
		}
		results := []res{}
		for _, t := range list {
			// 已是目标格式则跳过
			if t.Ext != "" && strings.EqualFold(strings.TrimPrefix(t.Ext, "."), target) {
				results = append(results, res{ID: t.ID, Before: t.FileName, After: t.FileName, OK: true})
				continue
			}
			dir := filepath.Dir(t.Path)
			base := stripAudioExt(filepath.Base(t.Path))
			dst := filepath.Join(dir, base+"."+target)
			if err := ffmpegConvert(t.Path, dst, codec); err != nil {
				results = append(results, res{ID: t.ID, Before: t.FileName, Error: err.Error()})
				continue
			}
			// 重新登记：删除原文件则替换曲目，否则新增一首
			if req.RemoveSource {
				s.remove(t.ID)
				os.Remove(t.Path)
			}
			refreshTrack(s, dst, fp)
			results = append(results, res{ID: t.ID, Before: t.FileName, After: filepath.Base(dst), OK: true})
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(results), "results": results})
	}
}

// ffmpegConvert re-encodes src to dst with the given audio codec args,
// keeping metadata tags via -map_metadata 0.
func ffmpegConvert(src, dst, codecArgs string) error {
	parts := strings.Fields(codecArgs)
	args := append([]string{"-y", "-i", src, "-map_metadata", "0", "-codec:a"}, parts...)
	args = append(args, dst)
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
