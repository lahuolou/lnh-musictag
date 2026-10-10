// 音乐解锁插件（tool.unlock）：把 Unlock Music 的解密能力接入 LNH-MusicTag。
//
// 接入方式（与 docs/PLUGINS.md 一致）：
//  1. 执行逻辑：本文件 + internal/unlock 包，编译进主程序（源码扩展点）；
//  2. 声明与启停：用户在「插件 → 导入插件 JSON」粘贴 unlock.plugin.json，
//     导入后插件页出现 tool.unlock，启用后才允许调用解锁接口；
//  3. 接口：POST /api/unlock（multipart 上传）→ GET /api/unlock/dl/{token}
//     下载 → POST /api/unlock/save 保存到曲库并重登记。
//
// 注意：/api/unlock 的请求体可能超过通用 8MiB 限制，因此挂在 limitBody
// 之外，但同样经过 requireAuth + csrfGuard。
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"LNH-musictag/internal/taglibx"
	"LNH-musictag/internal/unlock"
	cstore "LNH-musictag/internal/store"
)

// main 包 JSON/URL helper（避免与内部包重名）。
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }
func urlPathEscape(s string) string       { return url.PathEscape(s) }
func itoa(n int) string                   { return strconv.Itoa(n) }

const unlockPluginName = "tool.unlock"

// unlockTmpDir 存放解密后的临时文件（token.bin + token.json）。
var unlockTmpDir = filepath.Join(os.TempDir(), "lnh-unlock")

var unlockTokenRe = regexp.MustCompile(`^[a-f0-9]{64}$`)

// unlockPluginEnabled 检查 tool.unlock 插件是否已导入且启用。
func unlockPluginEnabled() bool {
	for _, p := range pluginsSnapshot() {
		if p.Name == unlockPluginName {
			return p.Enabled
		}
	}
	return false
}

// unlockMeta 是与临时文件配套的元数据。
type unlockMeta struct {
	Name   string   `json:"name"`   // 原始文件名（不含扩展名）
	Ext    string   `json:"ext"`    // 解密后扩展名
	Title  string   `json:"title"`
	Artist []string `json:"artist"`
	Album  string   `json:"album"`
	SongID string   `json:"songId"`
}

func unlockMetaPath(token string) string { return filepath.Join(unlockTmpDir, token+".json") }
func unlockDataPath(token string) string  { return filepath.Join(unlockTmpDir, token+".bin") }

func writeUnlockMeta(token string, m *unlockMeta) error {
	b, err := jsonMarshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(unlockMetaPath(token), b, 0o644)
}

func readUnlockMeta(token string) (*unlockMeta, error) {
	b, err := os.ReadFile(unlockMetaPath(token))
	if err != nil {
		return nil, err
	}
	m := &unlockMeta{}
	if err := jsonUnmarshal(b, m); err != nil {
		return nil, err
	}
	return m, nil
}

// unlockHandler POST /api/unlock：multipart 上传单个加密音乐文件，同步解密，
// 结果落临时目录并返回下载令牌与元数据。
func unlockHandler(cfg *cstore.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !unlockPluginEnabled() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "未启用「音乐解锁」插件：请先在插件页导入 unlock.plugin.json 并启用"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart: " + err.Error()})
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请选择要解锁的音乐文件（字段名 file）"})
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "读取上传文件失败: " + err.Error()})
			return
		}
		if len(data) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "上传文件为空"})
			return
		}
		res, err := unlock.Decrypt(data, hdr.Filename)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := os.MkdirAll(unlockTmpDir, 0o755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "临时目录创建失败: " + err.Error()})
			return
		}
		token := randomToken()
		if err := os.WriteFile(unlockDataPath(token), res.Data, 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "写入临时文件失败: " + err.Error()})
			return
		}
		meta := &unlockMeta{
			Name:   stripAudioExt(hdr.Filename),
			Ext:    res.Ext,
			Title:  res.Title,
			Artist: res.Artist,
			Album:  res.Album,
			SongID: res.SongID,
		}
		if err := writeUnlockMeta(token, meta); err != nil {
			os.Remove(unlockDataPath(token))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "写入元数据失败: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"token":    token,
			"fileName": hdr.Filename,
			"ext":      res.Ext,
			"size":     len(res.Data),
			"title":    res.Title,
			"artist":   res.Artist,
			"album":    res.Album,
			"songId":   res.SongID,
			"download": "/api/unlock/dl/" + token,
		})
	}
}

// unlockDownloadHandler GET /api/unlock/dl/{token}：下载解密后的文件。
func unlockDownloadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("token")
		if !unlockTokenRe.MatchString(token) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		meta, err := readUnlockMeta(token)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		data, err := os.ReadFile(unlockDataPath(token))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		name := meta.Name
		if name == "" {
			name = token
		}
		name = sanitizeName(name) + "." + meta.Ext
		w.Header().Set("Content-Type", unlock.MimeForExt(meta.Ext))
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+urlPathEscape(name))
		w.Header().Set("Content-Length", itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

// unlockSaveHandler POST /api/unlock/save：把已解密的临时文件保存到曲库目录，
// 可选写内嵌标签（NCM 元数据），并重新登记曲目。
// body: {"token":"...", "dir":"/music"（默认 scan_dir）, "writeTags":true}
func unlockSaveHandler(s *store, cfg *cstore.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token     string `json:"token"`
			Dir       string `json:"dir"` // 留空 = scan_dir
			WriteTags bool   `json:"writeTags"`
		}
		if err := jsonDecode(r.Body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !unlockTokenRe.MatchString(req.Token) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid token"})
			return
		}
		meta, err := readUnlockMeta(req.Token)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "解密结果不存在或已过期"})
			return
		}
		src := unlockDataPath(req.Token)
		data, err := os.ReadFile(src)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "解密结果不存在或已过期"})
			return
		}
		dir := strings.TrimSpace(req.Dir)
		if dir == "" {
			dir = cfg.GetDefault("scan_dir", "/music")
		}
		if !filepath.IsAbs(dir) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "保存目录必须是绝对路径"})
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "保存目录不可写: " + err.Error()})
			return
		}
		base := meta.Name
		if base == "" {
			base = "unlocked"
		}
		base = sanitizeName(base)
		dst := filepath.Join(dir, base+"."+meta.Ext)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "保存失败: " + err.Error()})
			return
		}
		// 写内嵌元数据（NCM meta；QMC songId 目前仅返回给前端，不做在线补齐）
		if req.WriteTags {
			tags := map[string][]string{}
			if meta.Title != "" {
				tags[taglibx.Title] = []string{meta.Title}
			}
			if len(meta.Artist) > 0 {
				tags[taglibx.Artist] = meta.Artist
			}
			if meta.Album != "" {
				tags[taglibx.Album] = []string{meta.Album}
			}
			if len(tags) > 0 {
				if err := taglibx.WriteTags(dst, tags, false); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "标签写入失败: " + err.Error(), "path": dst})
					return
				}
			}
		}
		// 清理临时文件
		os.Remove(src)
		os.Remove(unlockMetaPath(req.Token))
		refreshTrack(s, dst)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": dst, "ext": meta.Ext})
	}
}
