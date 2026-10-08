// Package dedup provides music-library deduplication.
//
// Strategy 1 (default, always on): identical-file detection by SHA256 content
// hash. Pure Go, fast, reliable for exact copies.
//
// Strategy 2: same-name / same-artist-title grouping so near-duplicates with
// different files (formats, rips) surface for the user to pick from.
package dedup

import (
	"path/filepath"
	"sort"
	"strings"

	"LNH-musictag/internal/model"
)

// formatTier ranks containers by lossless/quality potential (higher = better).
var formatTier = map[string]int{
	".flac": 6, ".wav": 6, ".aiff": 6, ".ape": 5, ".mpc": 5,
	".m4a": 4, ".ogg": 4, ".opus": 4, ".aac": 3, ".mp3": 3, ".mka": 3,
	".wma": 2,
}

// qualityScore gives a track a higher score the better its audio quality:
// format tier dominates, then bitrate, then sample rate.
func qualityScore(t *model.Track) int {
	tier := formatTier[strings.ToLower(t.Ext)]
	if tier == 0 {
		tier = 1
	}
	return tier*100000 + int(t.Bitrate)*100 + int(t.SampleRate)
}

// GroupByArtistTitle groups tracks whose ARTIST + TITLE tags match (normalized),
// falling back to the filename's "artist - title" pattern when a tag is empty.
// Catches the common "same song ripped twice with different filenames" case.
// The track with the best audio quality is flagged as the keeper (KeepID).
func GroupByArtistTitle(tracks []*model.Track) []model.DuplicateGroup {
	byKey := map[string][]*model.Track{}
	for _, t := range tracks {
		artist := strings.ToLower(strings.TrimSpace(t.Tags["ARTIST"]))
		title := strings.ToLower(strings.TrimSpace(t.Tags["TITLE"]))
		// 标签缺失/为空时，从文件名“艺术家 - 标题”解析补齐，
		// 保证空标签的库也能正确识别同歌不同名；文件名不同则自然不同组。
		if artist == "" || title == "" {
			fArtist, fTitle := parseArtistTitle(t.FileName)
			if artist == "" {
				artist = fArtist
			}
			if title == "" {
				title = fTitle
			}
		}
		if artist == "" || title == "" {
			continue
		}
		key := artist + "\x1f" + title
		byKey[key] = append(byKey[key], t)
	}
	groups := []model.DuplicateGroup{}
	for key, ts := range byKey {
		if len(ts) < 2 {
			continue
		}
		best := ts[0]
		for _, t := range ts[1:] {
			if qualityScore(t) > qualityScore(best) {
				best = t
			}
		}
		ids := make([]string, 0, len(ts))
		for _, t := range ts {
			ids = append(ids, t.ID)
		}
		parts := strings.Split(key, "\x1f")
		groups = append(groups, model.DuplicateGroup{
			Key: parts[0] + " - " + parts[1], Method: "tags", Count: len(ids), IDs: ids, KeepID: best.ID,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Count > groups[j].Count })
	return groups
}

// parseArtistTitle splits "艺术家 - 标题.ext" (and variants with 、/; separators)
// into (artist, title). It also strips trailing audio extensions from the whole
// name first, so "陈慧娴 - 傻女.flac" -> ("陈慧娴", "傻女").
func parseArtistTitle(fn string) (string, string) {
	base := strings.TrimSpace(fn)
	for {
		ext := strings.ToLower(filepath.Ext(base))
		if ext == "" || !model.AudioExts[ext] {
			break
		}
		base = strings.TrimSuffix(base, ext)
	}
	base = strings.TrimSpace(base)
	for _, sep := range []string{" - ", "-", " — ", "–"} {
		if i := strings.LastIndex(base, sep); i > 0 {
			artist := strings.TrimSpace(base[:i])
			title := strings.TrimSpace(base[i+len(sep):])
			if artist != "" && title != "" {
				return artist, title
			}
		}
	}
	return "", base
}

// cleanBaseName strips all trailing audio extensions and lowercases a filename,
// e.g. "广岛之恋.mp3.flac" -> "广岛之恋".
func cleanBaseName(fn string) string {
	base := fn
	for {
		ext := strings.ToLower(filepath.Ext(base))
		if ext == "" || !model.AudioExts[ext] {
			break
		}
		base = strings.TrimSuffix(base, ext)
	}
	return strings.TrimSpace(strings.ToLower(base))
}

// GroupBySameName groups tracks that share the same base name but are different
// formats (e.g. 广岛之恋.mp3 + 广岛之恋.flac). Among each group the track with
// the best audio quality is flagged as the keeper (KeepID). This catches the
// common "same song kept in multiple formats" duplication.
func GroupBySameName(tracks []*model.Track) []model.DuplicateGroup {
	byName := map[string][]*model.Track{}
	for _, t := range tracks {
		base := cleanBaseName(t.FileName)
		if base == "" {
			continue
		}
		byName[base] = append(byName[base], t)
	}
	groups := []model.DuplicateGroup{}
	for base, ts := range byName {
		exts := map[string]bool{}
		for _, t := range ts {
			exts[strings.ToLower(t.Ext)] = true
		}
		if len(exts) < 2 {
			continue
		}
		best := ts[0]
		for _, t := range ts[1:] {
			if qualityScore(t) > qualityScore(best) {
				best = t
			}
		}
		ids := make([]string, 0, len(ts))
		for _, t := range ts {
			ids = append(ids, t.ID)
		}
		groups = append(groups, model.DuplicateGroup{
			Key: base, Method: "format", Count: len(ids), IDs: ids, KeepID: best.ID,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Count > groups[j].Count })
	return groups
}

// GroupByHash groups tracks that share the same SHA256. Returns groups with
// more than one member, sorted by count desc.
func GroupByHash(tracks []*model.Track) []model.DuplicateGroup {
	byHash := map[string][]string{}
	for _, t := range tracks {
		if t.SHA256 == "" {
			continue
		}
		byHash[t.SHA256] = append(byHash[t.SHA256], t.ID)
	}
	return buildGroups(byHash, "hash")
}

func buildGroups(m map[string][]string, method string) []model.DuplicateGroup {
	// Non-nil empty slice so JSON serializes to [] (not null) when no dups.
	groups := []model.DuplicateGroup{}
	for key, ids := range m {
		if len(ids) < 2 {
			continue
		}
		g := model.DuplicateGroup{Key: key, Method: method, Count: len(ids), IDs: ids}
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Count > groups[j].Count })
	return groups
}
