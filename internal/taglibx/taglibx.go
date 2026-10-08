// Package taglibx wraps go.senan.xyz/taglib (WASM TagLib) to read and write
// audio metadata tags and embedded cover art across many formats, with no CGo.
package taglibx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.senan.xyz/taglib"

	"LNH-musictag/internal/model"
)

// ReadTrack reads tags, audio properties and cover presence for a single file
// and returns a model.Track with a SHA256 hash precomputed.
func ReadTrack(path string) (*model.Track, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	tags, err := taglib.ReadTags(path)
	if err != nil {
		return nil, fmt.Errorf("read tags: %w", err)
	}
	props, err := taglib.ReadProperties(path)
	if err != nil {
		return nil, fmt.Errorf("read properties: %w", err)
	}

	hash, err := sha256File(path)
	if err != nil {
		return nil, err
	}

	tr := &model.Track{
		Path:      path,
		FileName:  filepath.Base(path),
		Ext:       filepath.Ext(path),
		Size:      st.Size(),
		SHA256:    hash,
		Duration:  props.Length.Seconds(),
		Bitrate:   props.BitRate,
		SampleRate: props.SampleRate,
		Channels:  props.Channels,
		HasCover:  len(props.Images) > 0,
		HasLrcFile: LrcPath(path) != "",
		Tags:      flatten(tags),
	}
	tr.ID = pathID(path)
	return tr, nil
}

// LrcPath returns the sibling .lrc lyric file path for an audio file
// (case-insensitive), or "" if none exists. Chinese music libraries commonly
// keep lyrics as external .lrc files next to the audio.
func LrcPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	dir := filepath.Dir(path)
	for _, ext := range []string{".lrc", ".LRC", ".Lrc", ".lRC"} {
		p := filepath.Join(dir, base+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// pathID derives a stable, unique-per-path id (unlike SHA256, which groups
// identical files together — that grouping belongs to dedup, not identity).
func pathID(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}

// ReadCover returns the first embedded image as bytes (nil if none).
func ReadCover(path string) ([]byte, error) {
	img, err := taglib.ReadImage(path)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// WriteTags writes metadata tags. If clear is true, tags not present in tagsMap
// are removed. Values are multi-valued (slice) for fields like artist.
func WriteTags(path string, tagsMap map[string][]string, clear bool) error {
	opts := taglib.WriteOption(0)
	if clear {
		opts = taglib.Clear
	}
	return taglib.WriteTags(path, tagsMap, opts)
}

// WriteCover embeds imageBytes as front cover (index 0). Pass nil to clear.
func WriteCover(path string, imageBytes []byte) error {
	return taglib.WriteImage(path, imageBytes)
}

// Useful tag key aliases re-exported for convenience.
const (
	Title        = taglib.Title
	Artist       = taglib.Artist
	Album        = taglib.Album
	AlbumArtist  = taglib.AlbumArtist
	Genre        = taglib.Genre
	Date         = taglib.Date
	TrackNumber  = taglib.TrackNumber
	DiscNumber   = taglib.DiscNumber
	Lyrics       = taglib.Lyrics
	Comment      = taglib.Comment
	Composer     = taglib.Composer

	MBTrackID        = taglib.MusicBrainzTrackID
	MBAlbumID        = taglib.MusicBrainzAlbumID
	MBArtistID       = taglib.MusicBrainzArtistID
	MBReleaseGroupID = taglib.MusicBrainzReleaseGroupID
	MBReleaseTrackID = taglib.MusicBrainzReleaseTrackID
)

// SortedTagKeys returns the standardized keys present in a read tags map,
// sorted, excluding internal/technical keys, useful for UI rendering.
var technicalKeys = map[string]bool{
	"FILE": true, "FILETYPE": true, "LENGTH": true, "ORIGINALFILENAME": true,
	"METADATA_BLOCK_PICTURE": true,
}

// SortedTagKeys lists non-technical keys.
func SortedTagKeys(tags map[string][]string) []string {
	var out []string
	for k := range tags {
		if !technicalKeys[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func flatten(tags map[string][]string) map[string]string {
	out := make(map[string]string, len(tags))
	for k, vs := range tags {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
