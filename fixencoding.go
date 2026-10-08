// 乱码修复：针对中文标签常见的 GBK/UTF-8 编码错乱，把文本字段中的乱码还原。
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"LNH-musictag/internal/dedup"
	"LNH-musictag/internal/model"
	"LNH-musictag/internal/taglibx"
)

// textFields 是需要承载人读文本、参与乱码修复 / 简繁转换的标签字段。
var textFields = []string{
	taglibx.Title, taglibx.Artist, taglibx.Album, taglibx.AlbumArtist,
	taglibx.Genre, taglibx.Composer, taglibx.Comment, taglibx.Lyrics,
}

// resolveTracks converts an id list into the store's tracks; an empty list
// means all tracks.
func resolveTracks(s *store, ids []string) []*model.Track {
	if len(ids) == 0 {
		return s.all()
	}
	out := []*model.Track{}
	for _, id := range ids {
		if t := s.get(id); t != nil {
			out = append(out, t)
		}
	}
	return out
}

// cjkCount returns how many CJK (Han) characters are in s.
func cjkCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

// fixMojibake attempts to repair garbled Chinese tags caused by GBK/UTF-8
// encoding mixups. It scores each candidate by how much it looks like clean
// Chinese (more CJK, fewer non-CJK chars) and keeps the best, so already-good
// text is left untouched and only genuine mojibake is rewritten.
func fixMojibake(s string) string {
	if s == "" {
		return s
	}
	score := func(c string) int {
		total := utf8.RuneCountInString(c)
		cj := cjkCount(c)
		kana := 0
		for _, r := range c {
			if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
				kana++
			}
		}
		return cj*3 - (total-cj)*3 - kana*5
	}

	// 字符串全在 Latin-1 范围：很可能是 UTF-8/GBK 字节被按 Latin-1 读出。
	if latin, ok := latinBytesOf(s); ok {
		// 情形 C（优先、确定）：字节即原始 UTF-8，重组解码
		if utf8.Valid(latin) {
			return string(latin)
		}
		// 情形 B：字节是 GBK，按 GBK 解码
		if b, err := simplifiedchinese.GBK.NewDecoder().Bytes(latin); err == nil && score(string(b)) > score(s) {
			return string(b)
		}
		return s
	}

	// 含非 Latin-1 字符：尝试 GBK/UTF-8 错读还原，按得分取最优。
	best, bestScore := s, score(s)
	try := func(c string) {
		if sc := score(c); sc > bestScore {
			best, bestScore = c, sc
		}
	}
	// 情形 A：UTF-8 字节被按 GBK 读出（"浣犲ソ"）→ 再编码为 GBK、按 UTF-8 解码
	if b, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s)); err == nil && utf8.Valid(b) {
		try(string(b))
	}
	// 情形 B：GBK 字节被按 UTF-8 读出 → 按 GBK 解码
	if b, err := simplifiedchinese.GBK.NewDecoder().Bytes([]byte(s)); err == nil {
		try(string(b))
	}
	return best
}

// latinBytesOf returns s's runes as raw bytes when every rune is in 0x00-0xFF
// (i.e. the string looks Latin-1), plus ok=false otherwise.
func latinBytesOf(s string) ([]byte, bool) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			return nil, false
		}
		out = append(out, byte(r))
	}
	return out, true
}

// fixEncodingHandler rewrites the text fields of selected tracks, repairing
// any mojibake it can detect.
func fixEncodingHandler(s *store, fp dedup.Fingerprinter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ IDs []string `json:"ids"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		list := resolveTracks(s, req.IDs)
		type res struct {
			ID       string            `json:"id"`
			FileName string            `json:"fileName"`
			Changed  map[string]string `json:"changed"`
		}
		results := []res{}
		for _, t := range list {
			changed := map[string]string{}
			m := map[string][]string{}
			for _, k := range textFields {
				v := strings.TrimSpace(t.Tags[k])
				if v == "" {
					continue
				}
				f := fixMojibake(v)
				if f != v {
					changed[k] = f
					m[k] = []string{f}
				}
			}
			if len(m) > 0 {
				if taglibx.WriteTags(t.Path, m, false) == nil {
					refreshTrack(s, t.Path, fp)
				}
			}
			results = append(results, res{ID: t.ID, FileName: t.FileName, Changed: changed})
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(results), "results": results})
	}
}
