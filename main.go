// LNH-MusicTag prototype: a self-hosted web app for audio tag editing, library
// dedup and metadata scraping (MusicBrainz). Built with go.senan.xyz/taglib
// (WASM TagLib, no CGo) + pure-Go SHA256 dedup + optional fpcalc fingerprint.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"LNH-musictag/internal/dedup"
	"LNH-musictag/internal/model"
	"LNH-musictag/internal/scrape"
	"LNH-musictag/internal/taglibx"
)

//go:embed web/index.html
var webFS embed.FS

type store struct {
	mu     sync.RWMutex
	tracks map[string]*model.Track
	byPath map[string]string // path -> id
}

func newStore() *store {
	return &store{tracks: map[string]*model.Track{}, byPath: map[string]string{}}
}

func (s *store) add(t *model.Track) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byPath[t.Path]; exists {
		return
	}
	s.tracks[t.ID] = t
	s.byPath[t.Path] = t.ID
}

func (s *store) all() []*model.Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Track, 0, len(s.tracks))
	for _, t := range s.tracks {
		out = append(out, t)
	}
	return out
}

func (s *store) get(id string) *model.Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tracks[id]
}

func (s *store) exists(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.byPath[path]
	return ok
}

func main() {
	store := newStore()
	ms := scrape.NewMultiSource()
	if k := os.Getenv("ACOUSTID_API_KEY"); k != "" {
		ms.Client().AcoustIDAPIKey = k
	}

	// Optional fingerprint engine (fpcalc from Chromaprint). Noop if absent.
	var fingerprinter dedup.Fingerprinter = dedup.FpcalcFingerprinter{Bin: "fpcalc"}
	if !fingerprinter.Available() {
		fingerprinter = dedup.NoopFingerprinter{}
		log.Println("fingerprint engine (fpcalc) not found; dedup uses SHA256 only")
	}

	mux := http.NewServeMux()

	// Admin credentials. LNH_ADMIN_PASS set -> fixed; otherwise a random
	// initial password is generated and persisted to the config dir.
	adminUser := getenvDefault("LNH_ADMIN_USER", "admin")
	configDir := getenvDefault("LNH_CONFIG_DIR", "/config")
	configFile := filepath.Join(configDir, "admin.json")
	adminPass, generated := resolveAdminPassword(os.Getenv("LNH_ADMIN_PASS"), configFile)
	sessions := newSessionStore(adminUser, adminPass, configFile)
	jobs := newJobStore()

	mux.HandleFunc("POST /api/login", sessions.loginHandler)
	mux.HandleFunc("POST /api/logout", sessions.logoutHandler)
	mux.HandleFunc("GET /api/me", sessions.meHandler)

	// Protected API (requires login)
	pm := http.NewServeMux()
	pm.HandleFunc("POST /api/change-password", sessions.changePasswordHandler)
	pm.HandleFunc("POST /api/scrape/batch", batchScrapeHandler(jobs, store, ms, fingerprinter))
	pm.HandleFunc("GET /api/scrape/jobs/{id}", jobProgressHandler(jobs))
	pm.HandleFunc("GET /api/scrape/sources", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, ms.Sources())
	})
	pm.HandleFunc("POST /api/tracks/fix-title", fixTitleHandler(store, fingerprinter))
	mux.Handle("/api/", sessions.requireAuth(pm))

	// Static UI (catch-all root; more specific /api/... routes win)
	idx, _ := webFS.ReadFile("web/index.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(idx)
	})

	// Scan a directory
	pm.HandleFunc("POST /api/scan", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Dir string `json:"dir"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		dir := strings.TrimSpace(req.Dir)
		if dir == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dir is required"})
			return
		}
		added, err := scanDir(store, dir, fingerprinter)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"added": added, "total": len(store.all())})
	})

	// List tracks
	pm.HandleFunc("GET /api/tracks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.all())
	})

	// Duplicates (hash-based, plus fingerprint if available)
	pm.HandleFunc("GET /api/duplicates", func(w http.ResponseWriter, r *http.Request) {
		tracks := store.all()
		groups := dedup.GroupByHash(tracks)
		if fingerprinter.Available() {
			groups = append(groups, dedup.GroupByFingerprint(tracks)...)
		}
		writeJSON(w, http.StatusOK, groups)
	})

	// Serve embedded cover art of a track
	pm.HandleFunc("GET /api/tracks/{id}/cover", func(w http.ResponseWriter, r *http.Request) {
		t := store.get(r.PathValue("id"))
		if t == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "track not found"})
			return
		}
		img, err := taglibx.ReadCover(t.Path)
		if err != nil || len(img) == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no cover"})
			return
		}
		w.Header().Set("Content-Type", http.DetectContentType(img))
		w.Write(img)
	})

	// Update tags of a track
	pm.HandleFunc("POST /api/tracks/{id}/tags", func(w http.ResponseWriter, r *http.Request) {
		t := store.get(r.PathValue("id"))
		if t == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "track not found"})
			return
		}
		var req struct {
			Clear bool              `json:"clear"`
			Tags  map[string]string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		m := map[string][]string{}
		for k, v := range req.Tags {
			if strings.TrimSpace(v) != "" {
				m[strings.ToUpper(k)] = []string{v}
			}
		}
		if err := taglibx.WriteTags(t.Path, m, req.Clear); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		refreshTrack(store, t.Path, fingerprinter)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "written"})
	})

	// Embed cover art of a track (JSON body: {"url": "...", "dataBase64": "..."} or clear)
	pm.HandleFunc("POST /api/tracks/{id}/cover", func(w http.ResponseWriter, r *http.Request) {
		t := store.get(r.PathValue("id"))
		if t == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "track not found"})
			return
		}
		var req struct {
			URL        string `json:"url"`
			DataBase64 string `json:"dataBase64"`
			Clear      bool   `json:"clear"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var img []byte
		switch {
		case req.Clear:
			img = nil
		case req.DataBase64 != "":
			b, err := decodeBase64(req.DataBase64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			img = b
		case req.URL != "":
			b, err := fetchBytes(req.URL)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			img = b
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url, dataBase64 or clear required"})
			return
		}
		if err := taglibx.WriteCover(t.Path, img); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		refreshTrack(store, t.Path, fingerprinter)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "cover written"})
	})

	// Multi-source metadata search (source defaults to auto)
	pm.HandleFunc("GET /api/scrape/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		source := r.URL.Query().Get("source")
		limit := 8
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		results, err := ms.Search(source, q, limit)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, results)
	})

	// Fetch lyrics for a track (source optional; auto picks best provider)
	pm.HandleFunc("GET /api/lyrics", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sr := scrape.SearchResult{
			Source:   q.Get("source"),
			SourceID: q.Get("sourceId"),
			Title:    q.Get("title"),
		}
		if a := q.Get("artists"); a != "" {
			sr.Artists = strings.Split(a, "||")
		}
		if sr.Title == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
			return
		}
		lyric, err := ms.FetchLyrics(sr)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"lyric": lyric})
	})

	// Scrape a track: write metadata + cover from a chosen source result
	pm.HandleFunc("POST /api/tracks/{id}/scrape", func(w http.ResponseWriter, r *http.Request) {
		t := store.get(r.PathValue("id"))
		if t == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "track not found"})
			return
		}
		var req struct {
			Source      string   `json:"source"`
			SourceID    string   `json:"sourceId"`
			ReleaseMBID string   `json:"releaseMBID"`
			AlbumID     string   `json:"albumID"`
			Title       string   `json:"title"`
			Artists     []string `json:"artists"`
			Album       string   `json:"album"`
			AlbumArtist string   `json:"albumArtist"`
			Date        string   `json:"date"`
			CoverURL    string   `json:"coverURL"`
			FetchCover  bool     `json:"fetchCover"`
			FetchLyrics bool     `json:"fetchLyrics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		tags := map[string][]string{}
		if v := strings.TrimSpace(req.Title); v != "" {
			tags[taglibx.Title] = []string{v}
		}
		if len(req.Artists) > 0 {
			tags[taglibx.Artist] = req.Artists
		}
		if v := strings.TrimSpace(req.Album); v != "" {
			tags[taglibx.Album] = []string{v}
		}
		if v := strings.TrimSpace(req.AlbumArtist); v != "" {
			tags[taglibx.AlbumArtist] = []string{v}
		}
		if v := strings.TrimSpace(req.Date); v != "" {
			tags[taglibx.Date] = []string{v}
		}
		if v := strings.TrimSpace(req.SourceID); v != "" {
			switch req.Source {
			case "musicbrainz":
				tags[taglibx.MBTrackID] = []string{v}
			case "itunes":
				tags[taglibx.Comment] = []string{"itunes:" + v}
			case "netease":
				tags[taglibx.Comment] = []string{"netease:" + v}
			case "qq":
				tags[taglibx.Comment] = []string{"qq:" + v}
			}
		}
		if v := strings.TrimSpace(req.ReleaseMBID); v != "" {
			tags[taglibx.MBAlbumID] = []string{v}
		}
		if req.FetchLyrics && req.Title != "" {
			if lyric, err := ms.FetchLyrics(scrape.SearchResult{
				Source: req.Source, SourceID: req.SourceID,
				Title: req.Title, Artists: req.Artists,
			}); err == nil && strings.TrimSpace(lyric) != "" {
				tags[taglibx.Lyrics] = []string{lyric}
			}
		}
		if err := taglibx.WriteTags(t.Path, tags, false); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if req.FetchCover {
			if img, err := ms.FetchCover(scrape.SearchResult{
				Source: req.Source, ReleaseMBID: req.ReleaseMBID,
				CoverURL: req.CoverURL, AlbumID: req.AlbumID,
			}); err == nil && len(img) > 0 {
				taglibx.WriteCover(t.Path, img)
			}
		}
		refreshTrack(store, t.Path, fingerprinter)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "scraped"})
	})

	addr := ":10248"
	log.Printf("LNH-MusicTag prototype listening on http://localhost%s", addr)
	log.Printf("admin user: %q", adminUser)
	if generated {
		log.Printf("[首次启动] 已生成随机初始密码并保存到 %s：%s（请登录后在页面“修改密码”，或设置 LNH_ADMIN_PASS 环境变量固定）", configFile, adminPass)
	} else {
		log.Printf("admin password: 已配置（来自环境变量或配置文件，不在日志显示）")
	}
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// scanDir walks dir, adds audio files to the store and returns how many were
// added.
func scanDir(s *store, dir string, fp dedup.Fingerprinter) (int, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return 0, fmt.Errorf("cannot access %q: %w", dir, err)
	}
	if !st.IsDir() {
		return 0, fmt.Errorf("%q is not a directory", dir)
	}
	added := 0
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if !model.AudioExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		t, terr := taglibx.ReadTrack(path)
		if terr != nil {
			return nil
		}
		if fp.Available() {
			if f, ferr := fp.Fingerprint(path); ferr == nil {
				t.Fingerprint = f
			}
		}
		if !s.exists(path) {
			added++
		}
		s.add(t)
		return nil
	})
	return added, err
}

// fixTitleHandler strips trailing audio extensions from track titles in batch.
// Uses the selected ids (or all tracks when ids is empty). Writes TITLE back.
func fixTitleHandler(s *store, fp dedup.Fingerprinter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var list []*model.Track
		if len(req.IDs) == 0 {
			list = s.all()
		} else {
			for _, id := range req.IDs {
				if t := s.get(id); t != nil {
					list = append(list, t)
				}
			}
		}
		type fixRes struct {
			ID       string `json:"id"`
			FileName string `json:"fileName"`
			Fixed    bool   `json:"fixed"`
			Before   string `json:"before"`
			After    string `json:"after"`
		}
		results := []fixRes{}
		for _, t := range list {
			title := strings.TrimSpace(t.Tags["TITLE"])
			if title == "" {
				title = t.FileName
			}
			fixed := stripAudioExt(title)
			res := fixRes{ID: t.ID, FileName: t.FileName, Before: title, Fixed: fixed != title}
			if fixed != title {
				m := map[string][]string{taglibx.Title: {fixed}}
				if err := taglibx.WriteTags(t.Path, m, false); err == nil {
					refreshTrack(s, t.Path, fp)
					res.After = fixed
				} else {
					res.Fixed = false
					res.After = ""
					res.Before = "写标签失败: " + err.Error()
				}
			}
			results = append(results, res)
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(results), "fixed": results})
	}
}

// refreshTrack re-reads a track's tags/properties after a write.
func refreshTrack(s *store, path string, fp dedup.Fingerprinter) {
	t, err := taglibx.ReadTrack(path)
	if err != nil {
		return
	}
	if fp.Available() {
		if f, ferr := fp.Fingerprint(path); ferr == nil {
			t.Fingerprint = f
		}
	}
	s.mu.Lock()
	s.tracks[t.ID] = t
	s.byPath[path] = t.ID
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func decodeBase64(s string) ([]byte, error) {
	imported := s
	if i := strings.Index(s, ","); i >= 0 {
		imported = s[i+1:]
	}
	return decodeBase64Raw(imported)
}
