// Package unlock 是 Unlock Music 解密算法的纯 Go 移植。
//
// 支持：QMC 全系（qmc0/1/2/3/4/6/8/qmcflac/qmcogg/mflac*/mgg*/mmp4/tkm/
// bkc*/cache/tm2/tm6）、NCM + UC 缓存、KGM/VPR/KGMA、KWM、XM、MG3D、
// 喜马拉雅 X2M/X3M、QQ TM、JOOX（实验性，仅识别）。
// 全部算法内嵌，无外部依赖、无 wasm runtime，适合小体积低开销场景。
package unlock

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

// Result 是一次解锁的产物。
type Result struct {
	Data   []byte   // 解密后的音频数据
	Ext    string   // 目标扩展名（不带点）
	Title  string   // 内嵌标题（QMC QTag songId / NCM meta），可能为空
	Artist []string // 内嵌歌手（NCM meta）
	Album  string   // 内嵌专辑（NCM meta）
	SongID string   // QQ 音乐 songId（QTag），可用于后续在线补齐元数据
	RawExt string   // 原始扩展名（不带点）
}

var errNotAudio = errors.New("无法识别为有效音频文件")

// qmcHandlerExt 是 QMC 系目标格式映射（与上游 HandlerMap 一致）。
var qmcHandlerExt = map[string]string{
	"mgg": "ogg", "mgg0": "ogg", "mggl": "ogg", "mgg1": "ogg",
	"mflac": "flac", "mflac0": "flac", "mmp4": "mp4",
	"qmcflac": "flac", "qmcogg": "ogg",
	"qmc0": "mp3", "qmc2": "ogg", "qmc3": "mp3", "qmc4": "ogg", "qmc6": "ogg", "qmc8": "ogg",
	"bkcmp3": "mp3", "bkcm4a": "m4a", "bkcflac": "flac", "bkcwav": "wav",
	"bkcape": "ape", "bkcogg": "ogg", "bkcwma": "wma", "tkm": "m4a",
	"666c6163": "flac", "6d7033": "mp3", "6f6767": "ogg", "6d3461": "m4a", "776176": "wav",
}

// Decrypt 按文件名扩展名分发到对应解密器，返回解密产物。
func Decrypt(data []byte, filename string) (*Result, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	ext = strings.TrimPrefix(ext, ".")
	base := strings.TrimSuffix(filename, filepath.Ext(filename))

	switch ext {
	case "mg3d":
		out, err := DecryptMG3D(data)
		if err != nil {
			return nil, err
		}
		return finish(out, "wav", "", nil, "", "", ext)
	case "ncm":
		out, meta, err := DecryptNCM(data)
		if err != nil {
			return nil, err
		}
		return finishNCM(out, meta, base, ext)
	case "uc":
		out, err := DecryptNCMCache(data)
		if err != nil {
			return nil, err
		}
		return finish(out, "", "", nil, "", "", ext)
	case "kwm":
		out, err := DecryptKWM(data)
		if err != nil {
			return nil, err
		}
		return finish(out, "", "", nil, "", "", ext)
	case "xm", "wav", "mp3", "flac", "m4a":
		out, target, err := DecryptXM(data, ext)
		if err != nil {
			return nil, err
		}
		if target == "" {
			target = ext
		}
		return finish(out, target, "", nil, "", "", ext)
	case "ogg":
		return finish(data, "ogg", "", nil, "", "", ext)
	case "tm0", "tm3":
		return finish(data, "mp3", "", nil, "", "", ext)
	case "tm2", "tm6":
		out, err := DecryptTM(data)
		if err != nil {
			return nil, err
		}
		return finish(out, "m4a", "", nil, "", "", ext)
	case "cache":
		out, err := DecryptQMC(data, ext)
		if err != nil {
			return nil, err
		}
		return finish(out, "", "", nil, "", "", ext)
	case "vpr", "kgm", "kgma":
		out, err := DecryptKGM(data, ext == "vpr")
		if err != nil {
			return nil, err
		}
		return finish(out, "", "", nil, "", "", ext)
	case "ofl_en":
		if err := DecryptJOOX(data); err != nil {
			return nil, err
		}
		return nil, errors.New("JOOX 算法缺失，无法解锁")
	case "x2m", "x3m":
		out, err := DecryptXimalaya(data, ext)
		if err != nil {
			return nil, err
		}
		return finish(out, "", "", nil, "", "", ext)
	case "mflach":
		return nil, errors.New("mflach 网页版无法解锁，请使用 unlock-music CLI 版本")
	}
	// QMC 系（含 hex 后缀）
	if target, ok := qmcHandlerExt[ext]; ok {
		out, err := DecryptQMC(data, ext)
		if err != nil {
			return nil, err
		}
		return finish(out, target, "", nil, "", "", ext)
	}
	// 兜底：已是可播放音频则直通
	if e := SniffExt(data); e != "" {
		return finish(data, e, "", nil, "", "", ext)
	}
	return nil, errors.New("不支持此文件格式: " + ext)
}

func finishNCM(out []byte, meta *NcmMeta, base, rawExt string) (*Result, error) {
	if meta == nil {
		return finish(out, "", "", nil, "", "", rawExt)
	}
	title := meta.MusicName
	if title == "" {
		title = base
	}
	return finish(out, "", title, meta.Artist, meta.Album, "", rawExt)
}

func finish(out []byte, target, title string, artist []string, album, songID, rawExt string) (*Result, error) {
	if target == "" {
		target = SniffExt(out)
	}
	if target == "" {
		return nil, errNotAudio
	}
	return &Result{Data: out, Ext: target, Title: title, Artist: artist, Album: album, SongID: songID, RawExt: rawExt}, nil
}

// jsonUnmarshal 尽力解析 JSON（失败返回 err，调用方决定是否忽略）。
func jsonUnmarshal(b []byte, v any) error {
	return json.Unmarshal(b, v)
}

// MimeForExt 导出音频 MIME（供 API 层使用）。
func MimeForExt(ext string) string { return mimeForExt(ext) }
