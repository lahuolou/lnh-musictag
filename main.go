// LNH-MusicTag: a self-hosted web app for audio tag editing, library dedup,
// metadata scraping (domestic + international sources) and lyrics. Built with
// go.senan.xyz/taglib (WASM TagLib, no CGo) + pure-Go SHA256 dedup. Config
// (admin, API keys, options) is stored in SQLite, not environment variables.
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"LNH-musictag/internal/dedup"
	"LNH-musictag/internal/model"
	"LNH-musictag/internal/scrape"
	cstore "LNH-musictag/internal/store"
	"LNH-musictag/internal/taglibx"
)

//go:embed web/dist
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

func (s *store) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tracks[id]; ok {
		delete(s.tracks, id)
		delete(s.byPath, t.Path)
	}
}

func (s *store) removeByPath(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byPath[p]; ok {
		delete(s.tracks, id)
		delete(s.byPath, p)
	}
}

// scanState holds live progress of an asynchronous directory scan.
type scanState struct {
	mu      sync.Mutex
	Running bool   `json:"running"`
	Dir     string `json:"dir"`
	Added   int    `json:"added"`
	Total   int    `json:"total"` // audio files discovered so far
	Done    int    `json:"done"`
	Error   string `json:"error"`
}

// scanStatus is a lock-free snapshot of a scan state, safe to marshal/copy.
type scanStatus struct {
	Running bool   `json:"running"`
	Dir     string `json:"dir"`
	Added   int    `json:"added"`
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Error   string `json:"error"`
}

func (st *scanState) snapshot() scanStatus {
	st.mu.Lock()
	defer st.mu.Unlock()
	return scanStatus{
		Running: st.Running, Dir: st.Dir, Added: st.Added,
		Total: st.Total, Done: st.Done, Error: st.Error,
	}
}

type scanManager struct {
	mu    sync.Mutex
	state *scanState
}

func newScanManager() *scanManager { return &scanManager{} }

func (sm *scanManager) start(store *store, dir string, fp dedup.Fingerprinter, autoFix, autoRename bool) error {
	sm.mu.Lock()
	if sm.state != nil && sm.state.snapshot().Running {
		sm.mu.Unlock()
		return errors.New("扫描已在运行")
	}
	st := &scanState{Running: true, Dir: dir}
	sm.state = st
	sm.mu.Unlock()
	go func() {
		added, err := scanDirLive(store, dir, fp, st, autoFix, autoRename)
		st.mu.Lock()
		st.Running = false
		st.Added = added
		if err != nil {
			st.Error = err.Error()
		}
		st.mu.Unlock()
	}()
	return nil
}

func (sm *scanManager) status() scanStatus {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.state == nil {
		return scanStatus{}
	}
	return sm.state.snapshot()
}

func main() {
	store := newStore()
	ms := scrape.NewMultiSource()
	configDir := getenvDefault("LNH_CONFIG_DIR", "/config")

	// Database-backed config: admin + API keys live here, not in env vars.
	cfg, err := cstore.OpenConfig(configDir)
	if err != nil {
		log.Fatalf("open config db: %v", err)
	}
	defer cfg.Close()

	if k, ok := cfg.Get("acoustid_key"); ok && k != "" {
		ms.Client().AcoustIDAPIKey = k
	}

	// Optional fingerprint engine (fpcalc from Chromaprint). Noop if absent.
	var fingerprinter dedup.Fingerprinter = dedup.FpcalcFingerprinter{Bin: "fpcalc"}
	if !fingerprinter.Available() {
		fingerprinter = dedup.NoopFingerprinter{}
		log.Println("fingerprint engine (fpcalc) not found; dedup uses SHA256 only")
	}

	adminUser, adminPass, generated, err := bootstrapAdmin(cfg, os.Getenv("LNH_ADMIN_USER"), os.Getenv("LNH_ADMIN_PASS"))
	if err != nil {
		log.Fatalf("init admin: %v", err)
	}
	sessions := newSessionStore(cfg)
	jobs := newJobStore()
	scans := newScanManager()

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", sessions.loginHandler)
	mux.HandleFunc("POST /api/logout", sessions.logoutHandler)
	mux.HandleFunc("GET /api/me", sessions.meHandler)

	pm := http.NewServeMux()
	pm.HandleFunc("POST /api/change-password", sessions.changePasswordHandler)
	pm.HandleFunc("POST /api/scrape/batch", batchScrapeHandler(jobs, store, ms, fingerprinter))
	pm.HandleFunc("GET /api/scrape/jobs/{id}", jobProgressHandler(jobs))
	pm.HandleFunc("GET /api/scrape/sources", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, ms.Sources())
	})
	pm.HandleFunc("POST /api/tracks/fix-title", fixTitleHandler(store, fingerprinter))
	pm.HandleFunc("POST /api/rename-files", renameFilesHandler(store, fingerprinter))
	pm.HandleFunc("POST /api/convert", convertHandler(store, fingerprinter))
	pm.HandleFunc("POST /api/fix-encoding", fixEncodingHandler(store, fingerprinter))
	pm.HandleFunc("POST /api/convert-script", convertScriptHandler(store, fingerprinter))
	mux.Handle("/api/", sessions.requireAuth(pm))

	// Static UI (Vue SPA built into web/dist; /api/... routes win over this)
	dist, _ := fs.Sub(webFS, "web/dist")
	mux.Handle("/", http.FileServer(http.FS(dist)))

	// --- Scan (asynchronous, progressive) ---
	pm.HandleFunc("POST /api/scan", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Dir string `json:"dir"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		dir := strings.TrimSpace(req.Dir)
		if dir == "" {
			dir = cfg.GetDefault("scan_dir", "/music")
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "目录不可访问: " + dir})
			return
		}
		autoFix := cfg.GetDefault("auto_fix_title", "1") == "1"
		autoRename := cfg.GetDefault("auto_rename_file", "1") == "1"
		if err := scans.start(store, dir, fingerprinter, autoFix, autoRename); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": "扫描已开始", "dir": dir})
	})
	pm.HandleFunc("GET /api/scan/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, scans.status())
	})

	// --- Config / settings (stored in DB) ---
	pm.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"adminUser":      cfg.GetDefault("admin_user", "admin"),
			"acoustidKey":    cfg.GetDefault("acoustid_key", ""),
			"autoFixTitle":   cfg.GetDefault("auto_fix_title", "1") == "1",
			"autoRenameFile": cfg.GetDefault("auto_rename_file", "1") == "1",
			"scanDir":        cfg.GetDefault("scan_dir", "/music"),
		})
	})
	pm.HandleFunc("POST /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AdminUser      *string `json:"adminUser"`
			AcoustidKey    *string `json:"acoustidKey"`
			AutoFixTitle   *bool   `json:"autoFixTitle"`
			AutoRenameFile *bool   `json:"autoRenameFile"`
			ScanDir        *string `json:"scanDir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.AdminUser != nil {
			if u := strings.TrimSpace(*req.AdminUser); u != "" {
				cfg.Set("admin_user", u)
			}
		}
		if req.AcoustidKey != nil {
			cfg.Set("acoustid_key", strings.TrimSpace(*req.AcoustidKey))
			ms.Client().AcoustIDAPIKey = strings.TrimSpace(*req.AcoustidKey)
		}
		if req.AutoFixTitle != nil {
			v := "0"
			if *req.AutoFixTitle {
				v = "1"
			}
			cfg.Set("auto_fix_title", v)
		}
		if req.AutoRenameFile != nil {
			v := "0"
			if *req.AutoRenameFile {
				v = "1"
			}
			cfg.Set("auto_rename_file", v)
		}
		if req.ScanDir != nil {
			if d := strings.TrimSpace(*req.ScanDir); d != "" {
				cfg.Set("scan_dir", d)
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "已保存"})
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

	// Embed cover art of a track
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

	// Multi-source metadata search
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

	// Fetch lyrics for a track
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
			AlbumArtist []string `json:"albumArtist"`
			Genre       []string `json:"genre"`
			TrackNumber int      `json:"trackNumber"`
			Date        string   `json:"date"`
			CoverURL    string   `json:"coverURL"`
			FetchCover  bool     `json:"fetchCover"`
			FetchLyrics bool     `json:"fetchLyrics"`
			Fields      []string `json:"fields"` // 手动选择要写入的字段
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		fields := fieldsFromList(req.Fields, req.FetchCover, req.FetchLyrics)
		tags := map[string][]string{}
		if fields.Title {
			if v := stripAudioExt(strings.TrimSpace(req.Title)); v != "" {
				tags[taglibx.Title] = []string{v}
			}
		}
		if fields.Artist && len(req.Artists) > 0 {
			tags[taglibx.Artist] = req.Artists
		}
		if fields.Album {
			if v := strings.TrimSpace(req.Album); v != "" {
				tags[taglibx.Album] = []string{v}
			}
		}
		if fields.AlbumArtist && len(req.AlbumArtist) > 0 {
			tags[taglibx.AlbumArtist] = req.AlbumArtist
		}
		if fields.Genre && len(req.Genre) > 0 {
			tags[taglibx.Genre] = req.Genre
		}
		if fields.TrackNumber && req.TrackNumber > 0 {
			tags[taglibx.TrackNumber] = []string{strconv.Itoa(req.TrackNumber)}
		}
		if fields.Year {
			if v := strings.TrimSpace(req.Date); v != "" {
				tags[taglibx.Date] = []string{v}
			}
		}
		if v := strings.TrimSpace(req.SourceID); v != "" {
			switch req.Source {
			case "musicbrainz":
				tags[taglibx.MBTrackID] = []string{v}
			case "itunes", "netease", "qq", "kugou", "kuwo", "migu", "bilibili", "qishui", "qianqian":
				tags[taglibx.Comment] = []string{req.Source + ":" + v}
			}
		}
		if v := strings.TrimSpace(req.ReleaseMBID); v != "" {
			tags[taglibx.MBAlbumID] = []string{v}
		}
		if fields.Lyrics && req.Title != "" {
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
		if fields.Cover {
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
	log.Printf("LNH-MusicTag listening on http://localhost%s", addr)
	log.Printf("admin user: %q (config stored in %s/lnh.db)", adminUser, configDir)
	if generated {
		log.Printf("[首次启动] 已生成随机初始密码并保存到数据库：%s（登录后请在“设置”页修改密码）", adminPass)
	}
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// scanDirLive walks dir, adding audio files to the store progressively and
// updating the scan progress. When autoFix is enabled, trailing audio
// extensions in the title are stripped and written back to the file.
// When autoRename is enabled, files are first renamed to keep only their real
// encoding extension (detected from the header), e.g. "发如雪.mp3.flac" -> "发如雪.flac".
func scanDirLive(s *store, dir string, fp dedup.Fingerprinter, st *scanState, autoFix, autoRename bool) (int, error) {
	st0, err := os.Stat(dir)
	if err != nil {
		return 0, fmt.Errorf("cannot access %q: %w", dir, err)
	}
	if !st0.IsDir() {
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
		st.mu.Lock()
		st.Total++
		st.mu.Unlock()
		// 文件重命名：去重复/错误音频后缀（先校验真实编码格式）
		if autoRename {
			oldPath := path
			if np, rerr := renameCleanFile(path); rerr == nil && np != path {
				if s.exists(oldPath) {
					s.removeByPath(oldPath)
				}
				path = np
			}
		}
		t, terr := taglibx.ReadTrack(path)
		if terr != nil {
			st.mu.Lock()
			st.Done++
			st.mu.Unlock()
			return nil
		}
		// 默认去后缀：标题含音频扩展名时自动修正并写回文件
		if autoFix {
			title := strings.TrimSpace(t.Tags["TITLE"])
			if title == "" {
				title = t.FileName
			}
			if fixed := stripAudioExt(title); fixed != title {
				if taglibx.WriteTags(path, map[string][]string{taglibx.Title: {fixed}}, false) == nil {
					t, _ = taglibx.ReadTrack(path)
				}
			}
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
		st.mu.Lock()
		st.Added = s.allTracksCount()
		st.Done++
		st.mu.Unlock()
		return nil
	})
	return added, err
}

// allTracksCount returns the current number of tracks in the store.
func (s *store) allTracksCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tracks)
}

// fixTitleHandler strips trailing audio extensions from track titles in batch.
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

// renameFilesHandler renames selected (or all) tracks so their filename keeps
// only the real encoding extension, e.g. "发如雪.mp3.flac" -> "发如雪.flac".
func renameFilesHandler(s *store, fp dedup.Fingerprinter) http.HandlerFunc {
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
		type res struct {
			ID      string `json:"id"`
			Before  string `json:"before"`
			After   string `json:"after"`
			Renamed bool   `json:"renamed"`
			Error   string `json:"error,omitempty"`
		}
		results := []res{}
		for _, t := range list {
			np, err := renameCleanFile(t.Path)
			if err != nil {
				results = append(results, res{ID: t.ID, Before: t.FileName, Renamed: false, Error: err.Error()})
				continue
			}
			if np == t.Path {
				results = append(results, res{ID: t.ID, Before: t.FileName, Renamed: false})
				continue
			}
			// Re-register the track under its new path (its id derives from path).
			s.remove(t.ID)
			refreshTrack(s, np, fp)
			results = append(results, res{ID: t.ID, Before: t.FileName, After: filepath.Base(np), Renamed: true})
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(results), "results": results})
	}
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
