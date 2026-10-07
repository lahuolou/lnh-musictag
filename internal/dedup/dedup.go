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
	"sort"
	"strings"

	"LNH-musictag/internal/model"
)

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
	var groups []model.DuplicateGroup
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
