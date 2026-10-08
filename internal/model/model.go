// Package model defines the core data structures shared across the app.
package model

// AudioExts lists the audio file extensions the scanner accepts.
var AudioExts = map[string]bool{
	".mp3": true, ".flac": true, ".m4a": true, ".aac": true,
	".ogg": true, ".opus": true, ".wav": true, ".wma": true,
	".ape": true, ".mpc": true, ".aiff": true, ".mka": true,
}

// Track is a scanned audio file with its tags, audio properties and dedup info.
type Track struct {
	ID       string            `json:"id"`       // stable id: sha256[:16]
	Path     string            `json:"path"`     // absolute path on server
	FileName string            `json:"fileName"` // base name
	Ext      string            `json:"ext"`
	Size     int64             `json:"size"`
	SHA256   string            `json:"sha256"`   // full file hash (dedup strategy 1)
	Fingerprint string        `json:"fingerprint"` // chromaprint fingerprint, empty if unavailable
	Duration float64           `json:"duration"` // seconds
	Bitrate  uint              `json:"bitrate"`  // kbit/s
	SampleRate uint            `json:"sampleRate"`
	Channels uint              `json:"channels"`
	HasCover bool              `json:"hasCover"`
	HasLrcFile bool            `json:"hasLrcFile"` // 同目录存在外挂 .lrc 歌词文件
	HasTrad  bool              `json:"hasTrad"`   // 文本标签含繁体字（OpenCC 检测）
	Tags     map[string]string `json:"tags"`     // flattened: first value per standardized key
}

// DuplicateGroup groups tracks that share the same dedup key (sha256 or fingerprint).
type DuplicateGroup struct {
	Key    string   `json:"key"`
	Method string   `json:"method"` // "hash" | "fingerprint" | "format"
	Count  int      `json:"count"`
	IDs    []string `json:"ids"`
	KeepID string   `json:"keepId,omitempty"` // "format" 组的保留者（音质最佳）
}
