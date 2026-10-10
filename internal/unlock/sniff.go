package unlock

import "bytes"

// SniffExt 根据文件头嗅探音频容器格式（与上游 utils.SniffAudioExt 一致），
// 返回不带点的扩展名（mp3/flac/wav/m4a/ogg/opus），未知返回 ""。
func SniffExt(data []byte) string {
	if len(data) < 8 {
		return ""
	}
	switch {
	case len(data) > 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0:
		return "mp3"
	case len(data) >= 4 && bytes.Equal(data[0:4], []byte("fLaC")):
		return "flac"
	case len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")):
		return "wav"
	case len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")):
		// M4A/MP4：brand 位于 8..11
		typ := string(data[8:12])
		if typ == "M4A " || typ == "M4B " || typ == "M4P " || typ == "isom" || typ == "mp42" {
			return "m4a"
		}
		return "mp4"
	case len(data) >= 4 && bytes.Equal(data[0:4], []byte("OggS")):
		return "ogg"
	}
	return ""
}

// mime 映射（供 API 层使用）。
func mimeForExt(ext string) string {
	switch ext {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "m4a", "mp4":
		return "audio/mp4"
	case "ogg":
		return "audio/ogg"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "ape":
		return "audio/ape"
	case "wma":
		return "audio/wma"
	case "aiff":
		return "audio/aiff"
	}
	return "application/octet-stream"
}
