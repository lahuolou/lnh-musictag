package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"LNH-musictag/internal/model"
	"LNH-musictag/internal/scrape"
	"LNH-musictag/internal/taglibx"
)

// Batch scraping runs in a background job so the UI can show progress.
// Sources are rate-limited, so a short delay is applied between tracks when
// scraping more than one.

// ScrapeFields selects which metadata fields to write during scraping.
type ScrapeFields struct {
	Cover       bool
	Title       bool
	Artist      bool
	Album       bool
	AlbumArtist bool
	Genre       bool
	Year        bool
	TrackNumber bool
	Lyrics      bool
}

// allMetadataFields returns fields with every metadata tag enabled but no
// cover/lyrics (which are controlled separately by the caller's fetch flags).
func allMetadataFields() ScrapeFields {
	return ScrapeFields{
		Title: true, Artist: true, Album: true, AlbumArtist: true,
		Genre: true, Year: true, TrackNumber: true,
	}
}

// fieldsFromList converts an explicit list of selected field names (from the
// UI) into ScrapeFields. An empty list falls back to all metadata plus the
// legacy cover/lyrics fetch flags.
func fieldsFromList(list []string, fetchCover, fetchLyrics bool) ScrapeFields {
	if len(list) == 0 {
		f := allMetadataFields()
		f.Cover = fetchCover
		f.Lyrics = fetchLyrics
		return f
	}
	set := map[string]bool{}
	for _, s := range list {
		set[strings.ToLower(strings.TrimSpace(s))] = true
	}
	has := func(keys ...string) bool {
		for _, k := range keys {
			if set[k] {
				return true
			}
		}
		return false
	}
	return ScrapeFields{
		Cover:       has("cover", "海报", "封面"),
		Title:       has("title", "歌名"),
		Artist:      has("artist", "艺术家"),
		Album:       has("album", "专辑"),
		AlbumArtist: has("albumartist", "专辑艺术家"),
		Genre:       has("genre", "流派"),
		Year:        has("year", "年代"),
		TrackNumber: has("tracknumber", "曲目号"),
		Lyrics:      has("lyrics", "歌词"),
	}
}

// trackHasField reports whether a track already carries a value for field.
// field is a taglibx constant (e.g. taglibx.Title) or "cover"/"lyrics".
func trackHasField(t *model.Track, field string) bool {
	switch field {
	case "cover":
		return t.HasCover
	case "lyrics":
		return strings.TrimSpace(t.Tags["LYRICS"]) != ""
	default:
		return strings.TrimSpace(t.Tags[field]) != ""
	}
}

// applySkipFilled drops fields whose value the track already has. It returns
// true if at least one field remains to be written.
func applySkipFilled(t *model.Track, f *ScrapeFields) bool {
	if f.Title && trackHasField(t, taglibx.Title) {
		f.Title = false
	}
	if f.Artist && trackHasField(t, taglibx.Artist) {
		f.Artist = false
	}
	if f.Album && trackHasField(t, taglibx.Album) {
		f.Album = false
	}
	if f.AlbumArtist && trackHasField(t, taglibx.AlbumArtist) {
		f.AlbumArtist = false
	}
	if f.Genre && trackHasField(t, taglibx.Genre) {
		f.Genre = false
	}
	if f.Year && trackHasField(t, taglibx.Date) {
		f.Year = false
	}
	if f.TrackNumber && trackHasField(t, taglibx.TrackNumber) {
		f.TrackNumber = false
	}
	if f.Cover && trackHasField(t, "cover") {
		f.Cover = false
	}
	if f.Lyrics && trackHasField(t, "lyrics") {
		f.Lyrics = false
	}
	return f.Cover || f.Title || f.Artist || f.Album || f.AlbumArtist ||
		f.Genre || f.Year || f.TrackNumber || f.Lyrics
}

type scrapeJob struct {
	ID      string         `json:"id"`
	Kind    string         `json:"kind"` // scrape | convert | identify | fixEnc | script
	Total   int            `json:"total"`
	Done    int            `json:"done"`
	Current string         `json:"current"`
	Status  string         `json:"status"` // "running" | "done"
	Results []scrapeResult `json:"results"`
	// Updated carries tracks whose tags/cover changed since the last poll.
	// Each progress poll consumes (clears) it so the UI can patch the list
	// incrementally instead of reloading the whole list.
	Updated []*model.Track `json:"updated"`
	mu      sync.Mutex
}

type scrapeResult struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	OK       bool   `json:"ok"`
	Skip     bool   `json:"skip,omitempty"`
	Message  string `json:"message"`
}

type jobStore struct {
	mu   sync.Mutex
	jobs map[string]*scrapeJob
}

func newJobStore() *jobStore { return &jobStore{jobs: map[string]*scrapeJob{}} }

func (js *jobStore) create(total int, kind string) *scrapeJob {
	j := &scrapeJob{ID: randomToken(), Kind: kind, Total: total, Status: "running"}
	js.mu.Lock()
	js.jobs[j.ID] = j
	js.mu.Unlock()
	return j
}

func (js *jobStore) get(id string) *scrapeJob {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.jobs[id]
}

// active returns snapshots of all running jobs. The frontend calls this on
// login/refresh so tasks keep showing progress across page reloads and devices
// (the job itself runs in this process; a container restart cancels it).
func (js *jobStore) active() []*scrapeJob {
	js.mu.Lock()
	defer js.mu.Unlock()
	out := []*scrapeJob{}
	for _, j := range js.jobs {
		if j.Status != "running" {
			continue
		}
		j.mu.Lock()
		out = append(out, &scrapeJob{
			ID: j.ID, Kind: j.Kind, Total: j.Total, Done: j.Done,
			Current: j.Current, Status: j.Status, Results: append([]scrapeResult(nil), j.Results...),
		})
		j.mu.Unlock()
	}
	return out
}

// runProgressJob 启动通用后台任务（格式转换 / 音频识别 / 乱码修复 / 简繁转换）：
// 串行处理 total 项，每完成一项更新 Done/Current/Results，
// 前端通过 GET /api/jobs/{id} 轮询可视化进度。
func (js *jobStore) runProgressJob(kind string, total int, step func(j *scrapeJob, i int) scrapeResult) *scrapeJob {
	job := js.create(total, kind)
	go func() {
		for i := 0; i < total; i++ {
			r := step(job, i)
			job.mu.Lock()
			job.Done++
			job.Results = append(job.Results, r)
			job.mu.Unlock()
		}
		job.mu.Lock()
		job.Status = "done"
		job.Current = ""
		job.mu.Unlock()
	}()
	return job
}

// batchScrapeHandler starts a background job that scrapes the given track ids.
func batchScrapeHandler(js *jobStore, store *store, ms *scrape.MultiSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs         []string `json:"ids"`
			FetchCover  bool     `json:"fetchCover"`
			FetchLyrics bool     `json:"fetchLyrics"`
			Source      string   `json:"source"`
			Fields      []string `json:"fields"`
			SkipFilled  bool     `json:"skipFilled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if len(req.IDs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请至少勾选一首曲目"})
			return
		}
		if req.Source == "" {
			req.Source = "auto"
		}
		fields := fieldsFromList(req.Fields, req.FetchCover, req.FetchLyrics)
		job := js.create(len(req.IDs), "scrape")
		go runBatch(job, store, ms, req.Source, req.IDs, fields, req.SkipFilled)
		writeJSON(w, http.StatusOK, map[string]any{"jobId": job.ID, "total": job.Total})
	}
}

// activeJobsHandler lists all running jobs so the UI can resume polling after
// a page reload or when signing in from another device.
func activeJobsHandler(js *jobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"jobs": js.active()})
	}
}

// jobProgressHandler returns the current state of a batch job. Updated tracks
// are returned once and then cleared, so repeated polls deliver only the delta.
func jobProgressHandler(js *jobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		j := js.get(r.PathValue("id"))
		if j == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		j.mu.Lock()
		out := scrapeJob{
			ID:      j.ID,
			Total:   j.Total,
			Done:    j.Done,
			Current: j.Current,
			Status:  j.Status,
			Results: append([]scrapeResult(nil), j.Results...),
			Updated: j.Updated,
		}
		j.Updated = nil
		j.mu.Unlock()
		writeJSON(w, http.StatusOK, &out)
	}
}

func runBatch(job *scrapeJob, store *store, ms *scrape.MultiSource, source string, ids []string, fields ScrapeFields, skipFilled bool) {
	defer func() {
		job.mu.Lock()
		job.Status = "done"
		job.Current = ""
		job.mu.Unlock()
	}()
	multi := len(ids) > 1
	for _, id := range ids {
		t := store.get(id)
		if t == nil {
			job.mu.Lock()
			job.Results = append(job.Results, scrapeResult{ID: id, OK: false, Message: "曲目不存在"})
			job.Done++
			job.mu.Unlock()
			continue
		}
		job.mu.Lock()
		job.Current = t.FileName
		job.mu.Unlock()

		msg, ok := scrapeOneTrack(t, ms, source, fields, skipFilled, store)
		skip := strings.Contains(msg, "智能跳过")
		// 完成后把最新曲目快照加入增量队列（前端据此实时更新列表，不整表刷新）
		if nt := store.get(t.ID); nt != nil {
			job.mu.Lock()
			job.Updated = append(job.Updated, nt)
			job.Results = append(job.Results, scrapeResult{ID: t.ID, FileName: t.FileName, OK: ok, Skip: skip, Message: msg})
			job.Done++
			job.mu.Unlock()
		} else {
			job.mu.Lock()
			job.Results = append(job.Results, scrapeResult{ID: t.ID, FileName: t.FileName, OK: ok, Skip: skip, Message: msg})
			job.Done++
			job.mu.Unlock()
		}

		if multi {
			time.Sleep(1100 * time.Millisecond) // respect source rate limits
		}
	}
}

// scrapeOneTrack auto-scrapes a single track: search by its current title +
// artist on the given source, pick the best result, write only the enabled
// fields and optionally fetch the cover and/or lyrics. When skipFilled is set,
// fields the track already carries are left untouched (smart skip to save
// hardware); if nothing needs filling, the track is skipped without searching.
// Returns (message, ok).
func scrapeOneTrack(t *model.Track, ms *scrape.MultiSource, source string, fields ScrapeFields, skipFilled bool, store *store) (string, bool) {
	if skipFilled {
		// Drop already-filled fields; if nothing remains, skip without searching.
		if !applySkipFilled(t, &fields) {
			return "已有完整标签，智能跳过", true
		}
	}

	// 搜索关键词：优先真实标签；标签为空/“Unknown”/乱码时回退文件名解析
	// （如 `Unknown - 蓝色蝴蝶.mp3` → 用“蓝色蝴蝶”去刮削）。
	title, artist := effectiveSearchTerms(t)
	query := title
	if artist != "" && !looksUnknown(artist) && !looksGarbled(artist) {
		query += " " + artist
	}
	if strings.TrimSpace(query) == "" {
		return "无标题/艺术家，跳过", false
	}
	results, err := ms.Search(source, query, 3)
	if err != nil {
		return "搜索失败: " + err.Error(), false
	}
	if len(results) == 0 {
		return "无匹配结果", false
	}
	sr := pickBest(results)
	ms.Enrich(&sr) // 补全专辑艺术家/流派/年代/曲目号

	tags := map[string][]string{}
	if fields.Title && stripAudioExt(strings.TrimSpace(sr.Title)) != "" {
		tags[taglibx.Title] = []string{stripAudioExt(strings.TrimSpace(sr.Title))}
	}
	if fields.Artist && len(sr.Artists) > 0 {
		tags[taglibx.Artist] = sr.Artists
	}
	if fields.Album && sr.Album != "" {
		tags[taglibx.Album] = []string{sr.Album}
	}
	if fields.AlbumArtist && len(sr.AlbumArtist) > 0 {
		tags[taglibx.AlbumArtist] = sr.AlbumArtist
	}
	if fields.Genre && len(sr.Genre) > 0 {
		tags[taglibx.Genre] = sr.Genre
	}
	if fields.Year && sr.Date != "" {
		tags[taglibx.Date] = []string{sr.Date}
	}
	if fields.TrackNumber && sr.TrackNumber > 0 {
		tags[taglibx.TrackNumber] = []string{strconv.Itoa(sr.TrackNumber)}
	}
	if sr.Source == "musicbrainz" && sr.SourceID != "" {
		tags[taglibx.MBTrackID] = []string{sr.SourceID}
	}
	if sr.ReleaseMBID != "" {
		tags[taglibx.MBAlbumID] = []string{sr.ReleaseMBID}
	}
	if fields.Lyrics {
		if lyric, err := ms.FetchLyrics(sr); err == nil && strings.TrimSpace(lyric) != "" {
			tags[taglibx.Lyrics] = []string{lyric}
		}
	}
	if len(tags) > 0 {
		if err := taglibx.WriteTags(t.Path, tags, false); err != nil {
			return "写标签失败: " + err.Error(), false
		}
	}
	if fields.Cover {
		if img, err := ms.FetchCover(sr); err == nil && len(img) > 0 {
			taglibx.WriteCover(t.Path, img)
		}
	}
	refreshTrack(store, t.Path)

	var wrote []string
	if fields.Title {
		wrote = append(wrote, "歌名")
	}
	if fields.Cover {
		wrote = append(wrote, "海报")
	}
	if fields.Lyrics {
		wrote = append(wrote, "歌词")
	}
	msg := "刮削完成"
	if len(wrote) > 0 {
		msg += ": " + strings.Join(wrote, "+")
	}
	return msg, true
}

// pickBest chooses the first result that has an album, else the first result.
func pickBest(results []scrape.SearchResult) scrape.SearchResult {
	for i := range results {
		if results[i].Album != "" {
			return results[i]
		}
	}
	return results[0]
}
