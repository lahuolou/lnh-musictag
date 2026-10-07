package scrape

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// FetchLyrics returns the lyrics text for a result. Strategy:
//   - NetEase / QQ: fetch by the source song ID (best for Chinese tracks)
//   - otherwise: NetEase search by title+artist, then lrclib (international)
func (m *MultiSource) FetchLyrics(sr SearchResult) (string, error) {
	if sr.Source == "netease" && sr.SourceID != "" {
		if l, err := neteaseLyric(m.client, sr.SourceID); err == nil && l != "" {
			return l, nil
		}
	}
	if sr.Source == "qq" && sr.SourceID != "" {
		if l, err := qqLyric(m.client, sr.SourceID); err == nil && l != "" {
			return l, nil
		}
	}
	if sr.Title != "" {
		q := strings.TrimSpace(sr.Title + " " + strings.Join(sr.Artists, " "))
		res, err := neteaseSource{}.Search(m.client, q, 1)
		if err == nil && len(res) > 0 && res[0].SourceID != "" {
			if l, err2 := neteaseLyric(m.client, res[0].SourceID); err2 == nil && l != "" {
				return l, nil
			}
		}
		if l, err := lrclibLyric(m.client, sr.Title, sr.Artists); err == nil && l != "" {
			return l, nil
		}
	}
	return "", fmt.Errorf("未找到歌词")
}

// neteaseLyric fetches LRC lyrics by NetEase song id. NetEase returns a block
// (code -462) when Accept: application/json is sent, so use a plain request.
func neteaseLyric(c *Client, songID string) (string, error) {
	u := "https://music.163.com/api/song/lyric?id=" + url.PathEscape(songID) + "&lv=1&kv=1&tv=-1"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://music.163.com/")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var o struct {
		Code int `json:"code"`
		Lrc  struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&o); err != nil {
		return "", err
	}
	l := strings.TrimSpace(o.Lrc.Lyric)
	if o.Code != 200 || l == "" {
		return "", fmt.Errorf("netease lyric not available")
	}
	return l, nil
}

// qqLyric fetches lyrics by QQ song mid. The lyric may be base64-encoded.
func qqLyric(c *Client, songmid string) (string, error) {
	u := "https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=" +
		url.QueryEscape(songmid) + "&format=json&nobase64=1&songtype=0"
	headers := map[string]string{"Referer": "https://y.qq.com/"}
	var o struct {
		Retcode int    `json:"retcode"`
		Lyric   string `json:"lyric"`
	}
	if err := c.getJSON(u, headers, &o); err != nil {
		return "", err
	}
	if o.Lyric == "" {
		return "", fmt.Errorf("qq lyric not available")
	}
	if b, err := base64.StdEncoding.DecodeString(o.Lyric); err == nil && len(b) > 0 {
		return strings.TrimSpace(string(b)), nil
	}
	return strings.TrimSpace(o.Lyric), nil
}

// lrclibLyric fetches lyrics from lrclib (no key required, international).
func lrclibLyric(c *Client, title string, artists []string) (string, error) {
	q := strings.TrimSpace(title + " " + strings.Join(artists, " "))
	u := "https://lrclib.net/api/search?q=" + url.QueryEscape(q)
	var out []struct {
		Synced string `json:"syncedLyrics"`
		Plain  string `json:"plainLyrics"`
	}
	if err := c.getJSON(u, nil, &out); err != nil {
		return "", err
	}
	for _, it := range out {
		if it.Synced != "" {
			return it.Synced, nil
		}
	}
	for _, it := range out {
		if it.Plain != "" {
			return it.Plain, nil
		}
	}
	return "", fmt.Errorf("lrclib no lyrics")
}
