// Package scrape implements metadata enrichment against MusicBrainz and the
// Cover Art Archive (no API key required, subject to MusicBrainz rate limits)
// plus an optional AcoustID fingerprint lookup hook for when an API key and a
// fingerprint are available.
package scrape

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	mbBase    = "https://musicbrainz.org/ws/2"
	coverBase = "https://coverartarchive.org"
	userAgent = "LNH-MusicTag/0.1 (local prototype; contact: dev@example.invalid)"
)

// Client is a MusicBrainz scraper.
type Client struct {
	HTTP *http.Client
	// AcoustIDAPIKey, when non-empty, enables AcoustID recording lookup.
	AcoustIDAPIKey string
	// LastFMKey enables Last.fm track search.
	LastFMKey string
	// DiscogsToken enables Discogs database search.
	DiscogsToken string
	// JamendoClientID enables Jamendo track search.
	JamendoClientID string
	// SpotifyID/SpotifySecret enable Spotify search (client credentials).
	SpotifyID     string
	SpotifySecret string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// RecordingResult is a single search hit.
type RecordingResult struct {
	MBID       string    `json:"mbid"`
	Title      string    `json:"title"`
	Artists    []string  `json:"artists"`
	Duration   int       `json:"duration"` // ms, 0 if unknown
	Releases   []Release `json:"releases"`
	Score      int       `json:"score"`
	FirstDate  string    `json:"firstDate"`
}

// Release is a release associated with a recording.
type Release struct {
	MBID  string `json:"mbid"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// SearchRecordings queries MusicBrainz recordings by a lucene query string.
func (c *Client) SearchRecordings(query string, limit int) ([]RecordingResult, error) {
	if limit <= 0 || limit > 25 {
		limit = 8
	}
	u := fmt.Sprintf("%s/recording?query=%s&fmt=json&limit=%d",
		mbBase, url.QueryEscape(query), limit)
	body, err := c.get(u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Recordings []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Score   int    `json:"score"`
			Length  int    `json:"length"`
			ArtistCredit []struct {
				Name string `json:"name"`
			} `json:"artist-credit"`
			FirstReleaseDate string `json:"first-release-date"`
			Releases []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Date  string `json:"date"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse musicbrainz response: %w", err)
	}
	out := make([]RecordingResult, 0, len(resp.Recordings))
	for _, r := range resp.Recordings {
		rr := RecordingResult{
			MBID: r.ID, Title: r.Title, Duration: r.Length, Score: r.Score,
			FirstDate: r.FirstReleaseDate,
		}
		for _, ac := range r.ArtistCredit {
			if ac.Name != "" {
				rr.Artists = append(rr.Artists, ac.Name)
			}
		}
		for _, rel := range r.Releases {
			rr.Releases = append(rr.Releases, Release{MBID: rel.ID, Title: rel.Title, Date: rel.Date})
		}
		out = append(out, rr)
	}
	return out, nil
}

// BuildQuery builds a lucene search query from title and artist.
func BuildQuery(title, artist string) string {
	var parts []string
	if t := strings.TrimSpace(title); t != "" {
		parts = append(parts, fmt.Sprintf(`recording:"%s"`, escapeLucene(t)))
	}
	if a := strings.TrimSpace(artist); a != "" {
		parts = append(parts, fmt.Sprintf(`artist:"%s"`, escapeLucene(a)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

func escapeLucene(s string) string {
	r := strings.NewReplacer(`"`, `\"`, `\`, `\\`, `:`, `\:`, `[`, `\[`, `]`, `\]`)
	return r.Replace(s)
}

// ReleaseDetail holds enriched release metadata for album-artist, genres,
// release date and track numbers.
type ReleaseDetail struct {
	Artists []string
	Genres  []string
	Date    string
	Tracks  []ReleaseTrack
}

// ReleaseTrack is a track within a release.
type ReleaseTrack struct {
	Title  string
	Number string
}

// ReleaseDetail fetches a release's album-artist, genres, date and track list.
// Best-effort for metadata completion (genre/year/album artist/track number).
func (c *Client) ReleaseDetail(mbid string) (*ReleaseDetail, error) {
	u := fmt.Sprintf("%s/release/%s?inc=artist-credits+genres+recordings&fmt=json",
		mbBase, url.PathEscape(mbid))
	body, err := c.get(u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Date         string `json:"date"`
		ArtistCredit []struct {
			Name string `json:"name"`
		} `json:"artist-credit"`
		Genres []struct {
			Name string `json:"name"`
		} `json:"genres"`
		Media []struct {
			Tracks []struct {
				Title  string `json:"title"`
				Number string `json:"number"`
			} `json:"tracks"`
		} `json:"media"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	rd := &ReleaseDetail{Date: resp.Date}
	for _, ac := range resp.ArtistCredit {
		if ac.Name != "" {
			rd.Artists = append(rd.Artists, ac.Name)
		}
	}
	for _, g := range resp.Genres {
		if g.Name != "" {
			rd.Genres = append(rd.Genres, g.Name)
		}
	}
	for _, med := range resp.Media {
		for _, tr := range med.Tracks {
			rd.Tracks = append(rd.Tracks, ReleaseTrack{Title: tr.Title, Number: tr.Number})
		}
	}
	return rd, nil
}

// LookupByFingerprint identifies a recording from a Chromaprint fingerprint
// via the AcoustID API (requires AcoustIDAPIKey). Returns the best matching
// recording; error when the key is missing or nothing matches.
func (c *Client) LookupByFingerprint(fp string, durationSec int) (RecordingResult, error) {
	if c.AcoustIDAPIKey == "" {
		return RecordingResult{}, fmt.Errorf("AcoustID API Key 未设置（设置页填写）")
	}
	u := fmt.Sprintf("https://api.acoustid.org/v2/lookup?client=%s&fingerprint=%s&meta=recordings&format=json",
		url.QueryEscape(c.AcoustIDAPIKey), url.QueryEscape(fp))
	if durationSec > 0 {
		u += fmt.Sprintf("&duration=%d", durationSec)
	}
	body, err := c.get(u)
	if err != nil {
		return RecordingResult{}, err
	}
	var resp struct {
		Status  string `json:"status"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
		Results []struct {
			Score      float64 `json:"score"`
			Recordings []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Duration int    `json:"duration"`
				Artists  []struct {
					Name string `json:"name"`
				} `json:"artists"`
				ReleaseGroups []struct {
					FirstReleaseDate string `json:"first-release-date"`
					Title           string `json:"title"`
				} `json:"releasegroups"`
			} `json:"recordings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return RecordingResult{}, fmt.Errorf("parse acoustid response: %w", err)
	}
	if resp.Status != "ok" {
		msg := resp.Error.Message
		if msg == "" {
			msg = resp.Status
		}
		return RecordingResult{}, fmt.Errorf("acoustid: %s", msg)
	}
	best := RecordingResult{}
	for _, r := range resp.Results { // AcoustID 结果按分数降序
		for _, rec := range r.Recordings {
			if rec.Title == "" || len(rec.Artists) == 0 {
				continue
			}
			if best.MBID == "" { // 取第一个有效匹配（分数最高）
				best = RecordingResult{MBID: rec.ID, Title: rec.Title, Duration: rec.Duration}
				for _, a := range rec.Artists {
					if a.Name != "" {
						best.Artists = append(best.Artists, a.Name)
					}
				}
				for _, rg := range rec.ReleaseGroups {
					best.Releases = append(best.Releases, Release{MBID: "", Title: rg.Title, Date: rg.FirstReleaseDate})
				}
				if len(best.Releases) > 0 {
					best.FirstDate = best.Releases[0].Date
				}
			}
		}
	}
	if best.MBID == "" {
		return RecordingResult{}, fmt.Errorf("音频指纹未匹配到录音（AcoustID 无结果）")
	}
	return best, nil
}

// FetchCover retrieves the front cover for a release as bytes. Returns
// (nil, nil) if no cover exists.
func (c *Client) FetchCover(releaseMBID string) ([]byte, error) {
	u := fmt.Sprintf("%s/release/%s/front-500", coverBase, url.PathEscape(releaseMBID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover art archive status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) get(u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
