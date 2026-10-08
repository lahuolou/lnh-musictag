package scrape

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// SearchResult is the unified metadata hit returned by any scrape source.
type SearchResult struct {
	Source       string   `json:"source"`
	SourceID     string   `json:"sourceId"`
	ReleaseMBID  string   `json:"releaseMBID,omitempty"`
	AlbumID      string   `json:"albumID,omitempty"`
	Title        string   `json:"title"`
	Artists      []string `json:"artists"`
	Album        string   `json:"album"`
	AlbumArtist  []string `json:"albumArtist,omitempty"`
	Genre        []string `json:"genre,omitempty"`
	Date         string   `json:"date"`         // year (YYYY) or full date
	TrackNumber  int      `json:"trackNumber,omitempty"`
	Duration     int      `json:"duration"` // seconds
	CoverURL     string   `json:"coverURL,omitempty"`
}

// Source is a metadata provider. Search receives a plain text query.
type Source interface {
	Name() string
	Label() string
	Search(c *Client, query string, limit int) ([]SearchResult, error)
}

// SourceInfo is the public descriptor of a source for the UI dropdown.
type SourceInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// MultiSource aggregates domestic + international scrape sources.
type MultiSource struct {
	client *Client
	byName map[string]Source
	order  []string
}

// Source preference groups used by auto: Chinese queries hit domestic sources
// first, Latin-script queries hit international sources first.
var domesticOrder = []string{"netease", "qq", "kugou", "kuwo", "migu", "bilibili", "qishui", "bodian"}
var internationalOrder = []string{"musicbrainz", "itunes"}

// NewMultiSource registers all built-in sources (domestic first so the UI
// dropdown and auto mode both prefer Chinese services).
func NewMultiSource() *MultiSource {
	m := &MultiSource{client: NewClient(), byName: map[string]Source{}}
	m.register(neteaseSource{})
	m.register(qqSource{})
	m.register(kugouSource{})
	m.register(kuwoSource{})
	m.register(miguSource{})
	m.register(bilibiliSource{})
	m.register(qishuiSource{})
	m.register(bodianSource{})
	m.register(musicbrainzSource{})
	m.register(itunesSource{})
	return m
}

func (m *MultiSource) register(s Source) {
	if _, ok := m.byName[s.Name()]; !ok {
		m.byName[s.Name()] = s
		m.order = append(m.order, s.Name())
	}
}

// Client exposes the shared HTTP client for callers that need it.
func (m *MultiSource) Client() *Client { return m.client }

// Sources returns the ordered source list (for the UI dropdown).
func (m *MultiSource) Sources() []SourceInfo {
	out := []SourceInfo{{Name: "auto", Label: "自动"}}
	for _, n := range m.order {
		s := m.byName[n]
		out = append(out, SourceInfo{Name: n, Label: s.Label()})
	}
	return out
}

// hasCJK reports whether the query contains any CJK (Chinese) character.
func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// Search runs a source by name. "auto" (or empty) prefers sources by language:
// Chinese queries try domestic sources first, foreign/Latin queries try
// international sources first, falling back to the other group.
func (m *MultiSource) Search(name, query string, limit int) ([]SearchResult, error) {
	if name == "" {
		name = "auto"
	}
	if name != "auto" {
		s, ok := m.byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown source %q", name)
		}
		return s.Search(m.client, query, limit)
	}
	var order []string
	if hasCJK(query) {
		order = append(domesticOrder, internationalOrder...)
	} else {
		order = append(internationalOrder, domesticOrder...)
	}
	var lastErr error
	for _, n := range order {
		s, ok := m.byName[n]
		if !ok {
			continue
		}
		res, err := s.Search(m.client, query, limit)
		if err != nil {
			lastErr = err
			continue
		}
		if len(res) > 0 {
			return res, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return []SearchResult{}, nil
}

// Enrich fills album-artist, genre, release date and track number from the
// source's release detail when available (currently MusicBrainz). Best-effort;
// never returns an error.
func (m *MultiSource) Enrich(sr *SearchResult) {
	if sr == nil || sr.Source != "musicbrainz" || sr.ReleaseMBID == "" {
		return
	}
	rd, err := m.client.ReleaseDetail(sr.ReleaseMBID)
	if err != nil {
		return
	}
	if len(rd.Artists) > 0 {
		sr.AlbumArtist = rd.Artists
	}
	if len(rd.Genres) > 0 {
		sr.Genre = rd.Genres
	}
	if rd.Date != "" {
		sr.Date = rd.Date
	}
	for _, tr := range rd.Tracks {
		if strings.EqualFold(strings.TrimSpace(tr.Title), strings.TrimSpace(sr.Title)) {
			if n, e := strconv.Atoi(tr.Number); e == nil && n > 0 {
				sr.TrackNumber = n
			}
			break
		}
	}
}

// FetchCover fetches cover art bytes for a result according to its source.
func (m *MultiSource) FetchCover(sr SearchResult) ([]byte, error) {
	if sr.Source == "musicbrainz" && sr.ReleaseMBID != "" {
		return m.client.FetchCover(sr.ReleaseMBID)
	}
	if sr.Source == "netease" && sr.CoverURL == "" && sr.AlbumID != "" {
		return m.client.neteaseAlbumCover(sr.AlbumID)
	}
	if sr.CoverURL != "" {
		return m.client.getBytes(sr.CoverURL, nil)
	}
	return nil, nil
}

// neteaseAlbumCover resolves cover art via the NetEase album detail endpoint
// (the search API usually omits picUrl). Note: NetEase returns code -462 (a
// block) when Accept: application/json is sent, so this uses a plain request.
func (c *Client) neteaseAlbumCover(albumID string) ([]byte, error) {
	u := "https://music.163.com/api/album/" + url.PathEscape(albumID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://music.163.com/")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var o struct {
		Album struct {
			PicURL string `json:"picUrl"`
		} `json:"album"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&o); err != nil {
		return nil, err
	}
	if o.Album.PicURL == "" {
		return nil, nil
	}
	return c.getBytes(o.Album.PicURL+"?param=300y300", map[string]string{"Referer": "https://music.163.com/"})
}

// splitNames splits a combined artist/author string on ";", "、" and "/",
// trimming empty parts, e.g. "周杰伦;费玉清" -> ["周杰伦","费玉清"].
func splitNames(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ";") {
		for _, q := range strings.Split(p, "、") {
			q = strings.TrimSpace(q)
			if q != "" {
				out = append(out, q)
			}
		}
	}
	return out
}

// getJSON performs a GET and decodes a JSON body, setting extra headers.
func (c *Client) getJSON(u string, headers map[string]string, out any) error {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// getBytes performs a GET and returns the raw body bytes.
func (c *Client) getBytes(u string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
