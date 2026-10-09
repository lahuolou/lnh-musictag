// 插件设置接口：GET /api/plugins 列出全部插件（前端按类型分组渲染），
// POST /api/plugins 批量启停/配置并持久化、即时生效。
// 后续新增音源/下载/自动换源/Subsonic 协议等插件时，只需在启动时
// registerPlugin 注册描述与生效回调，前端与接口无需改动。
package main

import (
	"encoding/json"
	"net/http"

	"LNH-musictag/internal/scrape"
	cstore "LNH-musictag/internal/store"
)

// keyedSources 需要 API Key 的刮削源：源名 -> 设置库 Key 键名。
// 这些源默认关闭，需先在「设置 → 数据库配置」填写对应 Key 再开启。
var keyedSources = map[string]string{
	"lastfm":  "lastfm_key",
	"spotify": "spotify_id", // Spotify 需要 ID+Secret 两个 Key（KeyField 用 spotify_id）
	"discogs": "discogs_token",
	"jamendo": "jamendo_client_id",
}

// registerBuiltinPlugins 注册内置插件：
//  - scrape.*：现有刮削/音源（auto 内置不可停用；需 Key 的源默认关闭）
//  - tool.ffmpeg：FFmpeg 工具（可配置路径、可一键安装）
//  download / protocol 类型预留：后续插件直接在此追加注册。
func registerBuiltinPlugins(cfg *cstore.Config, ms *scrape.MultiSource) {
	for _, s := range ms.Sources() {
		name := "scrape." + s.Name
		builtin := s.Name == "auto"
		keyField := keyedSources[s.Name]
		// 需 Key 的源默认关闭（并且实际生效），配置 Key 前不参与自动刮削
		enabled := builtin || keyField == ""
		if !enabled {
			ms.SetEnabled(s.Name, false)
		}
		registerPlugin(&Plugin{
			Name:      name,
			Label:     s.Label,
			Kind:      PluginScrape,
			Enabled:   enabled,
			Builtin:   builtin,
			NeedsKey:  keyField != "",
			KeyField:  keyField,
		}, func(n string, on bool, _ map[string]string) {
			ms.SetEnabled(n[len("scrape."):], on)
		})
	}
	ff := &Plugin{
		Name:        "tool.ffmpeg",
		Label:       "FFmpeg",
		Kind:        PluginTool,
		Enabled:     true,
		Installable: true,
		Hint:        "ffHint",
		Fields: []PluginField{
			{Key: "path", Label: "ffPath", Type: "text", Placeholder: "ffPathPlaceholder"},
		},
	}
	registerPlugin(ff, nil)
	// 预留类型注册示例（后续实现时在此补充）：
	//   registerPlugin(&Plugin{Name:"download.example", Label:"…", Kind:PluginDownload,...})
	//   registerPlugin(&Plugin{Name:"protocol.subsonic", Label:"Subsonic", Kind:PluginProtocol,...})
	_ = cfg
}

// pluginsHandler GET 列出全部插件（含启停/配置/Key 状态），供插件页动态渲染。
func pluginsHandler(cfg *cstore.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := pluginsSnapshot()
		// 计算依赖 Key 的插件是否已配置（KeyConfigured）
		for i := range list {
			p := &list[i]
			if !p.NeedsKey || p.KeyField == "" || cfg == nil {
				continue
			}
			// Spotify 需要 ID + Secret 两个 Key
			if p.KeyField == "spotify_id" {
				a, _ := cfg.Get("spotify_id")
				b, _ := cfg.Get("spotify_secret")
				p.KeyConfigured = a != "" && b != ""
				continue
			}
			if v, ok := cfg.Get(p.KeyField); ok && v != "" {
				p.KeyConfigured = true
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"plugins": list})
	}
}

// pluginsUpdateHandler POST 批量更新插件状态：
// body: { "scrape.qq": {"enabled": false}, "tool.ffmpeg": {"config": {"path": "/usr/bin/ffmpeg"}} }
// 只更新传了 enabled / config 的字段；未知插件名直接报错。
func pluginsUpdateHandler(cfg *cstore.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req map[string]struct {
			Enabled *bool             `json:"enabled"`
			Config  map[string]string `json:"config"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
			return
		}
		byName := map[string]*Plugin{}
		for _, p := range pluginsSnapshot() {
			pp := p
			byName[pp.Name] = &pp
		}
		for name, upd := range req {
			p, ok := byName[name]
			if !ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown plugin: " + name})
				return
			}
			enabled := p.Enabled
			if upd.Enabled != nil {
				enabled = *upd.Enabled
			}
			if p.Builtin {
				enabled = true // 内置不可停用
			}
			applyPluginState(name, enabled, upd.Config)
		}
		savePluginsToDB(cfg)
		writeJSON(w, http.StatusOK, map[string]any{"plugins": pluginsSnapshot()})
	}
}
