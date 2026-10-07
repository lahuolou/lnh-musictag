package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"LNH-musictag/internal/dedup"
	"LNH-musictag/internal/model"
	"LNH-musictag/internal/scrape"
	"LNH-musictag/internal/taglibx"
)

// Batch scraping runs in a background job so the UI can show progress.
// Sources are rate-limited, so a short delay is applied between tracks when
// scraping more than one.

type scrapeJob struct {
	ID      string         `json:"id"`
	Total   int            `json:"total"`
	Done    int            `json:"done"`
	Current string         `json:"current"`
	Status  string         `json:"status"` // "running" | "done"
	Results []scrapeResult `json:"results"`
	mu      sync.Mutex
}

type scrapeResult struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

type jobStore struct {
	mu   sync.Mutex
	jobs map[string]*scrapeJob
}

func newJobStore() *jobStore { return &jobStore{jobs: map[string]*scrapeJob{}} }

func (js *jobStore) create(total int) *scrapeJob {
	j := &scrapeJob{ID: randomToken(), Total: total, Status: "running"}
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

// batchScrapeHandler starts a background job that scrapes the given track ids.
func batchScrapeHandler(js *jobStore, store *store, ms *scrape.MultiSource, fp dedup.Fingerprinter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs         []string `json:"ids"`
			FetchCover  bool     `json:"fetchCover"`
			FetchLyrics bool     `json:"fetchLyrics"`
			Source      string   `json:"source"`
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
		job := js.create(len(req.IDs))
		go runBatch(job, store, ms, fp, req.Source, req.IDs, req.FetchCover, req.FetchLyrics)
		writeJSON(w, http.StatusOK, map[string]any{"jobId": job.ID, "total": job.Total})
	}
}

// jobProgressHandler returns the current state of a batch job.
func jobProgressHandler(js *jobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		j := js.get(r.PathValue("id"))
		if j == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		writeJSON(w, http.StatusOK, j)
	}
}

func runBatch(job *scrapeJob, store *store, ms *scrape.MultiSource, fp dedup.Fingerprinter, source string, ids []string, fetchCover, fetchLyrics bool) {
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

		msg, ok := scrapeOneTrack(t, ms, source, fetchCover, fetchLyrics, store, fp)
		job.mu.Lock()
		job.Results = append(job.Results, scrapeResult{ID: t.ID, FileName: t.FileName, OK: ok, Message: msg})
		job.Done++
		job.mu.Unlock()

		if multi {
			time.Sleep(1100 * time.Millisecond) // respect source rate limits
		}
	}
}

// scrapeOneTrack auto-scrapes a single track: search by its current title +
// artist on the given source, pick the best result, write tags and optionally
// fetch the cover and/or lyrics. Returns (message, ok).
func scrapeOneTrack(t *model.Track, ms *scrape.MultiSource, source string, fetchCover, fetchLyrics bool, store *store, fp dedup.Fingerprinter) (string, bool) {
	title := strings.TrimSpace(t.Tags["TITLE"])
	artist := strings.TrimSpace(t.Tags["ARTIST"])
	query := strings.TrimSpace(title + " " + artist)
	if query == "" {
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

	tags := map[string][]string{taglibx.Title: {sr.Title}}
	if len(sr.Artists) > 0 {
		tags[taglibx.Artist] = sr.Artists
	}
	if sr.Album != "" {
		tags[taglibx.Album] = []string{sr.Album}
	}
	if sr.Date != "" {
		tags[taglibx.Date] = []string{sr.Date}
	}
	if sr.Source == "musicbrainz" && sr.SourceID != "" {
		tags[taglibx.MBTrackID] = []string{sr.SourceID}
	}
	if sr.ReleaseMBID != "" {
		tags[taglibx.MBAlbumID] = []string{sr.ReleaseMBID}
	}
	if fetchLyrics {
		if lyric, err := ms.FetchLyrics(sr); err == nil && strings.TrimSpace(lyric) != "" {
			tags[taglibx.Lyrics] = []string{lyric}
		}
	}
	if err := taglibx.WriteTags(t.Path, tags, false); err != nil {
		return "写标签失败: " + err.Error(), false
	}
	if fetchCover {
		if img, err := ms.FetchCover(sr); err == nil && len(img) > 0 {
			taglibx.WriteCover(t.Path, img)
		}
	}
	refreshTrack(store, t.Path, fp)
	album := ""
	if sr.Album != "" {
		album = " / " + sr.Album
	}
	extra := ""
	if fetchLyrics {
		extra = " +歌词"
	}
	return "刮削完成: " + sr.Title + album + extra, true
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
