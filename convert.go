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
)

// convertCodecs maps target extension -> ffmpeg audio codec args.
var convertCodecs = map[string]string{
	"mp3":  "libmp3lame -q:a 2",
	"flac": "flac",
	"m4a":  "aac -b:a 192k",
	"ogg":  "libvorbis -q:a 5",
	"opus": "libopus -b:a 128k",
	"wav":  "pcm_s16le",
	"aiff": "pcm_s16be",
	"aac":  "aac -b:a 192k",
	"wma":  "wmav2 -b:a 192k",
	"tta":  "tta",
}

// losslessCodecs ignore bitrate settings.
var losslessCodecs = map[string]bool{"flac": true, "wav": true, "aiff": true, "tta": true}

// convertHandler transcodes selected tracks to a target format.
func convertHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs          []string `json:"ids"`
			Target       string   `json:"target"`
			InputFormats []string `json:"inputFormats"` // 留空=全部；按源文件扩展名过滤
			Bitrate      string   `json:"bitrate"`      // 如 192k / keep / ""
			SampleRate   string   `json:"sampleRate"`   // 如 44100 / keep / ""
			RemoveSource bool     `json:"removeSource"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		inSet := map[string]bool{}
		for _, f := range req.InputFormats {
			f = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(f, ".")))
			if f != "" {
				inSet[f] = true
			}
		}
		br := strings.ToLower(strings.TrimSpace(req.Bitrate))
		if br == "keep" {
			br = ""
		}
		sr := strings.ToLower(strings.TrimSpace(req.SampleRate))
		if sr == "keep" {
			sr = ""
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
			srcExt := strings.ToLower(strings.TrimPrefix(t.Ext, "."))
			// 输入格式过滤
			if len(inSet) > 0 && !inSet[srcExt] {
				continue
			}
			target := strings.ToLower(strings.TrimSpace(req.Target))
			if target == "keep" || target == "" {
				target = srcExt
			}
			codec, ok := convertCodecs[target]
			if !ok {
				results = append(results, res{ID: t.ID, Before: t.FileName, Error: "不支持的目标格式: " + target})
				continue
			}
			// 已是目标格式：仍可按比特率/采样率重编码
			same := strings.EqualFold(srcExt, target)
			dir := filepath.Dir(t.Path)
			base := stripAudioExt(filepath.Base(t.Path))
			dst := filepath.Join(dir, base+"."+target)
			if err := ffmpegConvert(t.Path, dst, codec, br, sr, losslessCodecs[target]); err != nil {
				results = append(results, res{ID: t.ID, Before: t.FileName, Error: err.Error()})
				continue
			}
			// 重新登记：删除原文件则替换曲目，否则新增一首
			if req.RemoveSource && !same {
				s.remove(t.ID)
				os.Remove(t.Path)
			}
			refreshTrack(s, dst)
			results = append(results, res{ID: t.ID, Before: t.FileName, After: filepath.Base(dst), OK: true})
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(results), "results": results})
	}
}

// ffmpegConvert re-encodes src to dst with the given audio codec args,
// keeping metadata tags via -map_metadata 0.
func ffmpegConvert(src, dst, codecArgs, bitrate, sampleRate string, lossless bool) error {
	parts := strings.Fields(codecArgs)
	args := []string{"-y", "-i", src, "-map_metadata", "0", "-codec:a"}
	args = append(args, parts...)
	if bitrate != "" && !lossless {
		args = append(args, "-b:a", bitrate)
	}
	if sampleRate != "" {
		args = append(args, "-ar", sampleRate)
	}
	args = append(args, dst)
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
