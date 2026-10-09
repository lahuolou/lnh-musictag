// 乱码修复：针对中文标签常见的 GBK/UTF-8 编码错乱，把文本字段中的乱码还原。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

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
// text is left untouched and only genuine mojibake is rewritten. It also tries
// double-pass chains (e.g. GBK->UTF8->GBK->UTF8) that produce the classic
// "锟斤拷" corruption.
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
		if c != "" && c != s && utf8.ValidString(c) && !strings.ContainsRune(c, '\uFFFD') {
			if sc := score(c); sc > bestScore {
				best, bestScore = c, sc
			}
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
	// 双重链：s -> GBK 编码 -> UTF-8 解码（串含 FFFD）-> 再 GBK 编码 -> UTF-8 解码
	// 覆盖"锟斤拷"类二次损坏（中间有替换字符丢失，能还原的大部分仍可救回）。
	if b1, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s)); err == nil {
		if mid, err2 := simplifiedchinese.GBK.NewDecoder().Bytes(b1); err2 == nil {
			if b2, err3 := simplifiedchinese.GBK.NewEncoder().Bytes(mid); err3 == nil {
				if out, err4 := simplifiedchinese.GBK.NewDecoder().Bytes(b2); err4 == nil {
					try(string(out))
				}
			}
		}
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

// looksGarbled reports whether a text field shows typical mojibake markers:
// replacement chars, Latin-1 misread characters, bopomofo or geometric symbols
// that never appear in a clean Chinese tag.
func looksGarbled(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsRune(s, '\uFFFD') {
		return true
	}
	for _, r := range s {
		// 注音符号 U+3100–312F（ㄅㄆㄇ…）
		if r >= 0x3100 && r <= 0x312F {
			return true
		}
		// 几何图形 U+25A0–25FF（◈★▣）与杂项符号 U+2600–26FF（☀♫）
		if (r >= 0x25A0 && r <= 0x25FF) || (r >= 0x2600 && r <= 0x26FF) {
			return true
		}
		if r >= 0x80 && r <= 0xA0 { // Latin-1 控制/符号区（Ã/â 等误读特征）
			return true
		}
	}
	return false
}

// parseFileNameArtistTitle splits "艺术家 - 标题.ext" from a filename, falling
// back to (base, "") when no separator is present. Used to recover readable
// artist/title from an unrecoverable garbled tag.
func parseFileNameArtistTitle(fn string) (artist, title string) {
	base := fn
	for {
		ext := strings.ToLower(filepath.Ext(base))
		if ext == "" || !model.AudioExts[ext] {
			break
		}
		base = strings.TrimSuffix(base, ext)
	}
	base = strings.TrimSpace(base)
	// 文件名规范为「艺术家 - 歌曲标题」，标题本身可含“-”，因此按首个分隔符拆分
	for _, sep := range []string{" - ", "—", "–", "-"} {
		if i := strings.Index(base, sep); i > 0 {
			a := strings.TrimSpace(base[:i])
			t := strings.TrimSpace(base[i+len(sep):])
			if a != "" && t != "" {
				return a, t
			}
		}
	}
	return "", base
}

// looksUnknown reports whether a tag value is a placeholder that carries no
// real metadata ("Unknown", "未知", 空值等)。刮削搜索时这些值没有检索价值。
func looksUnknown(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	switch strings.ToLower(s) {
	case "unknown", "unknown artist", "unknown artists", "未知", "未知艺术家", "未知歌手", "<unknown>", "n/a", "none":
		return true
	}
	return false
}

// effectiveSearchTerms returns the title/artist that should be used to search
// for a track: real tag values when present, otherwise values parsed from the
// filename. This lets files like "Unknown - 蓝色蝴蝶.mp3" (placeholder artist,
// clean title) still be scraped by their actual song name. If a tag is empty,
// "Unknown" or clearly garbled, the filename wins.
func effectiveSearchTerms(t *model.Track) (title, artist string) {
	title = strings.TrimSpace(t.Tags["TITLE"])
	artist = strings.TrimSpace(t.Tags["ARTIST"])
	if title == "" || looksUnknown(title) || looksGarbled(title) ||
		artist == "" || looksUnknown(artist) || looksGarbled(artist) {
		fa, ft := parseFileNameArtistTitle(t.FileName) // 返回 (artist, title)
		if title == "" || looksUnknown(title) || looksGarbled(title) {
			title = ft
		}
		if artist == "" || looksUnknown(artist) || looksGarbled(artist) {
			artist = fa
		}
	}
	return title, artist
}

// fixEncodingHandler rewrites the text fields of selected tracks, repairing
// any mojibake it can detect. When a tag cannot be repaired (double-corrupted,
// information lost) but the filename carries a clean "artist - title", the
// artist/title tags fall back to the filename so the track stays readable.
// Runs as a background job for live progress.
func fixEncodingHandler(js *jobStore, s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ IDs []string `json:"ids"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		list := resolveTracks(s, req.IDs)
		if len(list) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请至少勾选一首曲目"})
			return
		}
		job := js.runProgressJob("fixEnc", len(list), func(j *scrapeJob, i int) scrapeResult {
			t := list[i]
			j.mu.Lock()
			j.Current = t.FileName
			j.mu.Unlock()
			changed := map[string]string{}
			m := map[string][]string{}
			for _, k := range textFields {
				v := strings.TrimSpace(t.Tags[k])
				if v == "" {
					continue
				}
				f := fixMojibake(v)
				// 有明确乱码特征（◈/注音/FFFD/Latin-1 控制区）的字段：文件名解析结果
				// 远比“尽力修复”可信，标题/艺术家直接回退文件名；其他字段仍尽力修复。
				if looksGarbled(v) && (k == taglibx.Title || k == taglibx.Artist) {
					fArtist, fTitle := parseFileNameArtistTitle(t.FileName)
					if k == taglibx.Title && fTitle != "" {
						f = fTitle
					} else if k == taglibx.Artist && fArtist != "" {
						f = fArtist
					}
					if f == v { // 文件名解析不出有效值，退回尽力修复
						f = fixMojibake(v)
					}
				}
				// 修复结果仍含替换符/乱码特征时丢弃，避免以乱易乱
				if f != v && !strings.ContainsRune(f, '\uFFFD') && !looksGarbled(f) {
					changed[k] = f
					m[k] = []string{f}
				}
			}
			if len(m) > 0 {
				if err := taglibx.WriteTags(t.Path, m, false); err != nil {
					return scrapeResult{ID: t.ID, FileName: t.FileName, OK: false, Message: "写标签失败: " + err.Error()}
				}
				refreshTrack(s, t.Path)
				return scrapeResult{ID: t.ID, FileName: t.FileName, OK: true, Message: fmt.Sprintf("修复 %d 个字段", len(changed))}
			}
			// 无乱码/无可修复内容：中性跳过，不算失败、不标红
			return scrapeResult{ID: t.ID, FileName: t.FileName, Skip: true, Message: "无变化，已跳过"}
		})
		writeJSON(w, http.StatusOK, map[string]any{"jobId": job.ID, "total": job.Total})
	}
}
