package scrape

import (
	"net/url"
	"strconv"
)

// qianqianSource searches Qianqian Music (千千音乐, ex-Baidu Music / Ting).
type qianqianSource struct{}

func (qianqianSource) Name() string  { return "qianqian" }
func (qianqianSource) Label() string { return "千千音乐" }

func (qianqianSource) Search(c *Client, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 25 {
		limit = 8
	}
	u := "https://tingapi.ting.baidu.com/v1/restserver/ting?method=baidu.ting.search.common&query=" +
		url.QueryEscape(query) + "&page_num=1&page_size=" + strconv.Itoa(limit) + "&format=json"
	var resp struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			SongList []struct {
				SongID      string `json:"song_id"`
				Title       string `json:"title"`
				Author      string `json:"author"`
				AlbumTitle  string `json:"album_title"`
				AlbumID     string `json:"album_id"`
				ReleaseDate string `json:"publishtime"`
			} `json:"song_list"`
		} `json:"result"`
	}
	if err := c.getJSON(u, nil, &resp); err != nil {
		return nil, err
	}
	out := []SearchResult{}
	for _, s := range resp.Result.SongList {
		out = append(out, SearchResult{
			Source: "qianqian", SourceID: s.SongID, Title: s.Title,
			Artists: splitNames(s.Author), Album: s.AlbumTitle,
			AlbumID: s.AlbumID, Date: s.ReleaseDate,
		})
	}
	return out, nil
}
