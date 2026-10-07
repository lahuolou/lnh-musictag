package scrape

import (
	"net/url"
	"strconv"
)

// miguSource searches Migu Music (咪咕音乐) web search API.
type miguSource struct{}

func (miguSource) Name() string  { return "migu" }
func (miguSource) Label() string { return "咪咕音乐" }

func (miguSource) Search(c *Client, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 25 {
		limit = 8
	}
	u := "https://music.migu.cn/v3/api/music/audio/search?keyword=" +
		url.QueryEscape(query) + "&pgc=1&rows=" + strconv.Itoa(limit) + "&searchSwitch=%7B%7D"
	headers := map[string]string{"Referer": "https://music.migu.cn/"}
	var resp struct {
		Code string `json:"code"`
		Data struct {
			Result []struct {
				SongID  string `json:"songId"`
				Title   string `json:"title"`
				Singer  []struct {
					Name string `json:"name"`
				} `json:"singer"`
				Album struct {
					AlbumID   string `json:"albumId"`
					AlbumName string `json:"albumName"`
					PicList   []struct {
						Img string `json:"img"`
					} `json:"picList"`
				} `json:"album"`
				Duration int `json:"duration"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.getJSON(u, headers, &resp); err != nil {
		return nil, err
	}
	out := []SearchResult{}
	for _, s := range resp.Data.Result {
		artists := []string{}
		for _, sg := range s.Singer {
			if sg.Name != "" {
				artists = append(artists, sg.Name)
			}
		}
		cover := ""
		if len(s.Album.PicList) > 0 {
			cover = s.Album.PicList[0].Img
		}
		out = append(out, SearchResult{
			Source: "migu", SourceID: s.SongID, Title: s.Title,
			Artists: artists, Album: s.Album.AlbumName,
			AlbumID: s.Album.AlbumID, Duration: s.Duration, CoverURL: cover,
		})
	}
	return out, nil
}
