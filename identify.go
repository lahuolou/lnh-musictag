// 音频内容识别：对“无文件名信息、无标签”的曲目（如 track01.mp3）用
// Chromaprint 音频指纹与库内已标注曲目做相似度匹配，命中后把标签复制
// 过去，并尝试抓取歌词，让这类文件也能自动补全。
package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"LNH-musictag/internal/finger"
	"LNH-musictag/internal/model"
	"LNH-musictag/internal/scrape"
	"LNH-musictag/internal/taglibx"
)

// filenameNoInfo reports whether a filename carries no usable title info:
// empty, placeholder ("Unknown"), garbled, or a bare track number (01, track01).
// A placeholder artist (Unknown/未知) also counts as "no info": the file
// has no real artist attribution and should go through audio identification.
func filenameNoInfo(fn string) bool {
	artist, title := parseFileNameArtistTitle(fn)
	t := strings.TrimSpace(title)
	if t == "" || looksUnknown(t) || looksGarbled(t) {
		return true
	}
	if looksUnknown(artist) {
		return true
	}
	low := strings.ToLower(t)
	low = strings.TrimPrefix(low, "track")
	if low != "" {
		allDigit := true
		for _, r := range low {
			if r < '0' || r > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return true
		}
	}
	return false
}

// needsIdentify reports whether a track has no usable metadata at all: a
// filename that carries no real title (track01/01/Unknown/乱码/空) whose tags
// are either empty or just taglib's filename fallback (title == basename).
// Only then does the track need audio-content identification.
func needsIdentify(t *model.Track) bool {
	if !filenameNoInfo(t.FileName) {
		return false
	}
	title := strings.TrimSpace(t.Tags["TITLE"])
	if title != "" && !looksUnknown(title) && !looksGarbled(title) {
		base := strings.TrimSuffix(filepath.Base(t.FileName), filepath.Ext(t.FileName))
		if !strings.EqualFold(strings.ToLower(title), strings.ToLower(base)) {
			return false // 标签是真实信息（如 01.mp3 内嵌“七里香”）
		}
	}
	return true
}

// identifyBase caches fingerprints of tagged tracks so repeated identify runs
// do not re-analyze the whole library (keyed by id; built once per process).
var (
	identifyBaseOnce sync.Once
	identifyBase     = map[string]string{} // trackID -> fingerprint
)

func buildIdentifyBase(s *store, fp *finger.Fpcalc) {
	identifyBaseOnce.Do(func() {
		tracks := s.all()
		workers := runtime.NumCPU()
		if workers > 4 {
			workers = 4
		}
		if workers < 1 {
			workers = 1
		}
		ch := make(chan *model.Track)
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for t := range ch {
					title := strings.TrimSpace(t.Tags["TITLE"])
					if title == "" || looksUnknown(title) {
						continue
					}
					if f, err := fp.Fingerprint(t.Path); err == nil && f != "" {
						identifyBase[t.ID] = f
					}
				}
			}()
		}
		for _, t := range tracks {
			ch <- t
		}
		close(ch)
		wg.Wait()
	})
}

type identifyResult struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	OK       bool   `json:"ok"`
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Message  string `json:"message"`
}

// identifyHandler matches selected (or all unknown) tracks against the tagged
// library by audio fingerprint, copies the matched tags and fetches lyrics.
// Runs as a background job with per-track progress.
func identifyHandler(js *jobStore, s *store, ms *scrape.MultiSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		fp := &finger.Fpcalc{Bin: "fpcalc"}
		if !fp.Available() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "音频识别引擎不可用：容器内需安装 chromaprint（fpcalc），请更新镜像"})
			return
		}

		var targets []*model.Track
		if len(req.IDs) > 0 {
			targets = resolveTracks(s, req.IDs)
		} else {
			for _, t := range s.all() {
				if needsIdentify(t) {
					targets = append(targets, t)
				}
			}
		}
		if len(targets) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"jobId": "", "total": 0})
			return
		}

		buildIdentifyBase(s, fp)

		job := js.runProgressJob("identify", len(targets), func(j *scrapeJob, i int) scrapeResult {
			t := targets[i]
			j.mu.Lock()
			j.Current = t.FileName
			j.mu.Unlock()
			f, err := fp.Fingerprint(t.Path)
			if err != nil {
				return scrapeResult{ID: t.ID, FileName: t.FileName, OK: false, Message: "指纹计算失败: " + err.Error()}
			}
			res := matchAndWrite(s, ms, t, f, identifyBase)
			if res.OK {
				return scrapeResult{ID: t.ID, FileName: t.FileName, OK: true, Message: res.Message}
			}
			// 未匹配/匹配来源无标题：属“未识别”，中性跳过，不算失败、不标红；
			// 仅写标签失败等真错误计入失败。
			skip := strings.Contains(res.Message, "未匹配") || strings.Contains(res.Message, "无标题")
			return scrapeResult{ID: t.ID, FileName: t.FileName, OK: false, Skip: skip, Message: res.Message}
		})
		writeJSON(w, http.StatusOK, map[string]any{"jobId": job.ID, "total": job.Total})
	}
}

// matchAndWrite finds the best fingerprint match, copies tags and fetches
// lyrics for a track. It returns the per-track result.
func matchAndWrite(s *store, ms *scrape.MultiSource, t *model.Track, fp string, base map[string]string) identifyResult {
	bestID, bestSim := "", 0.0
	for id, f := range base {
		if id == t.ID {
			continue
		}
		if sim := finger.Similarity(fp, f); sim > bestSim {
			bestID, bestSim = id, sim
		}
	}
	const threshold = 0.85
	var title, artist string
	var src *model.Track
	if bestID != "" && bestSim >= threshold {
		src = s.get(bestID)
		if src != nil {
			title = strings.TrimSpace(src.Tags["TITLE"])
			artist = strings.TrimSpace(src.Tags["ARTIST"])
		}
	}
	if src == nil {
		// 库内未匹配到 → 外部 AcoustID 听声识曲（需设置页填 AcoustID API Key）
		rec, aerr := ms.Client().LookupByFingerprint(fp, int(t.Duration))
		if aerr == nil && rec.Title != "" && len(rec.Artists) > 0 {
			return writeIdentify(s, ms, t, rec.Title, rec.Artists, rec.Releases, rec.FirstDate)
		}
		msg := "音频内容未匹配到库内曲目"
		if aerr != nil && strings.Contains(aerr.Error(), "API Key") {
			msg += "；如需外部听声识曲，请在设置页填写 AcoustID API Key"
		}
		return identifyResult{ID: t.ID, FileName: t.FileName, OK: false, Message: msg}
	}
	if title == "" {
		return identifyResult{ID: t.ID, FileName: t.FileName, OK: false, Message: "匹配来源无标题"}
	}
	tags := map[string][]string{taglibx.Title: {title}}
	if artist != "" {
		tags[taglibx.Artist] = []string{artist}
	}
	for k, v := range map[string]string{
		taglibx.Album:       src.Tags["ALBUM"],
		taglibx.AlbumArtist: src.Tags["ALBUMARTIST"],
		taglibx.Genre:       src.Tags["GENRE"],
		taglibx.Date:        src.Tags["DATE"],
		taglibx.TrackNumber: src.Tags["TRACKNUMBER"],
	} {
		if strings.TrimSpace(v) != "" {
			tags[k] = []string{v}
		}
	}
	if err := taglibx.WriteTags(t.Path, tags, false); err != nil {
		return identifyResult{ID: t.ID, FileName: t.FileName, OK: false, Message: "写标签失败: " + err.Error()}
	}
	// 抓歌词
	sr := scrape.SearchResult{Title: title}
	if artist != "" {
		sr.Artists = []string{artist}
	}
	if lyric, err := ms.FetchLyrics(sr); err == nil && strings.TrimSpace(lyric) != "" {
		taglibx.WriteTags(t.Path, map[string][]string{taglibx.Lyrics: {lyric}}, false)
	}
	refreshTrack(s, t.Path)
	return identifyResult{ID: t.ID, FileName: t.FileName, OK: true, Title: title, Artist: artist, Message: "已识别：" + title + (func() string { if artist != "" { return " - " + artist }; return "" })()}
}

// writeIdentify applies an AcoustID-recognized recording to a track and
// fetches lyrics for it.
func writeIdentify(s *store, ms *scrape.MultiSource, t *model.Track, title string, artists []string, releases []scrape.Release, date string) identifyResult {
	tags := map[string][]string{taglibx.Title: {title}, taglibx.Artist: artists}
	if len(releases) > 0 && strings.TrimSpace(releases[0].Title) != "" {
		tags[taglibx.Album] = []string{releases[0].Title}
	}
	if strings.TrimSpace(date) != "" {
		tags[taglibx.Date] = []string{date}
	}
	if err := taglibx.WriteTags(t.Path, tags, false); err != nil {
		return identifyResult{ID: t.ID, FileName: t.FileName, OK: false, Message: "写标签失败: " + err.Error()}
	}
	sr := scrape.SearchResult{Title: title, Artists: artists}
	if lyric, err := ms.FetchLyrics(sr); err == nil && strings.TrimSpace(lyric) != "" {
		taglibx.WriteTags(t.Path, map[string][]string{taglibx.Lyrics: {lyric}}, false)
	}
	refreshTrack(s, t.Path)
	return identifyResult{ID: t.ID, FileName: t.FileName, OK: true, Title: title, Artist: strings.Join(artists, " / "), Message: "已识别：" + title + " - " + strings.Join(artists, " / ")}
}
