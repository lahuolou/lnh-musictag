package scrape

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SearchResult is the unified metadata hit returned by any scrape source.
type SearchResult struct {
	Source      string   `json:"source"`
	SourceID    string   `json:"sourceId"`
	ReleaseMBID string   `json:"releaseMBID,omitempty"`
	AlbumID     string   `json:"albumID,omitempty"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	Date        string   `json:"date"`
	Duration    int      `json:"duration"` // seconds
	CoverURL    string   `json:"coverURL,omitempty"`
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

// NewMultiSource registers all built-in sources.
func NewMultiSource() *MultiSource {
	m := &MultiSource{client: NewClient(), byName: map[string]Source{}}
	m.register(musicbrainzSource{})
	m.register(itunesSource{})
	m.register(neteaseSource{})
	m.register(qqSource{})
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
	out := []SourceInfo{{Name: "auto", Label: "自动（依次尝试）"}}
	for _, n := range m.order {
		s := m.byName[n]
		out = append(out, SourceInfo{Name: n, Label: s.Label()})
	}
	return out
}

// Search runs a source by name. "auto" (or empty) tries all sources in order
// until one returns results.
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
	var lastErr error
	for _, n := range m.order {
		res, err := m.byName[n].Search(m.client, query, limit)
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

// FetchCover fetches cover art bytes for a result according to its source.
// Returns (nil, nil) when the source has no cover.
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
