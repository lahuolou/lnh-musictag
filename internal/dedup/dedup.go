// Package dedup provides music-library deduplication.
//
// Strategy 1 (default, always on): identical-file detection by SHA256 content
// hash. Pure Go, fast, reliable for exact copies.
//
// Strategy 2 (pluggable): acoustic fingerprinting via Chromaprint/fpcalc. The
// Fingerprinter interface is implemented by FpcalcFingerprinter when the
// fpcalc binary is present, otherwise it is a NoopFingerprinter. This keeps the
// prototype buildable on Windows without a C toolchain (Chromaprint normally
// requires CGo + native decode libs).
package dedup

import (
	"os/exec"
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

// Fingerprinter computes an audio fingerprint for a file path.
type Fingerprinter interface {
	// Available reports whether the underlying engine can be invoked.
	Available() bool
	// Fingerprint returns a chromaprint fingerprint string for the file, or an
	// error if unavailable or failed.
	Fingerprint(path string) (string, error)
}

// NoopFingerprinter is the default when no engine (fpcalc) is installed.
type NoopFingerprinter struct{}

func (NoopFingerprinter) Available() bool                    { return false }
func (NoopFingerprinter) Fingerprint(string) (string, error) { return "", nil }

// FpcalcFingerprinter shells out to the Chromaprint fpcalc CLI, which the user
// may install separately. Kept out of CGo so the project builds without a C
// toolchain.
type FpcalcFingerprinter struct{ Bin string }

func (f FpcalcFingerprinter) Available() bool {
	if f.Bin == "" {
		return false
	}
	_, err := exec.LookPath(f.Bin)
	return err == nil
}

func (f FpcalcFingerprinter) Fingerprint(path string) (string, error) {
	out, err := exec.Command(f.Bin, "-plain", path).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GroupByFingerprint groups tracks by their fingerprint (only tracks that have
// one). Returns groups with more than one member.
func GroupByFingerprint(tracks []*model.Track) []model.DuplicateGroup {
	byFp := map[string][]string{}
	for _, t := range tracks {
		if t.Fingerprint == "" {
			continue
		}
		byFp[t.Fingerprint] = append(byFp[t.Fingerprint], t.ID)
	}
	return buildGroups(byFp, "fingerprint")
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
