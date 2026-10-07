package scrape

import (
	"net/url"
	"strconv"
	"strings"
)

// kuwoSource searches Kuwo Music (酷我音乐) via the public r.s search gateway.
type kuwoSource struct{}

func (kuwoSource) Name() string  { return "kuwo" }
func (kuwoSource) Label() string { return "酷我音乐" }

func (kuwoSource) Search(c *Client, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 25 {
		limit = 8
	}
	u := "https://search.kuwo.cn/r.s?all=" + url.QueryEscape(query) +
		"&ft=music&itemset=web_2013&client=kt&pn=0&rn=" + strconv.Itoa(limit) +
		"&rformat=json&encoding=utf8"
	var resp struct {
		MUSICDATA []struct {
			SONGNAME string `json:"SONGNAME"`
			ARTIST   string `json:"ARTIST"`
			ALBUM    string `json:"ALBUM"`
			MUSICRID string `json:"MUSICRID"`
			DURATION string `json:"DURATION"`
		} `json:"MUSICDATA"`
	}
	if err := c.getJSON(u, nil, &resp); err != nil {
		return nil, err
	}
	out := []SearchResult{}
	for _, s := range resp.MUSICDATA {
		id := strings.TrimPrefix(s.MUSICRID, "MUSIC_")
		secs := 0
		if n, e := strconv.Atoi(s.DURATION); e == nil {
			secs = n
		}
		out = append(out, SearchResult{
			Source: "kuwo", SourceID: id, Title: s.SONGNAME,
			Artists: splitNames(s.ARTIST), Album: s.ALBUM, Duration: secs,
		})
	}
	return out, nil
}
