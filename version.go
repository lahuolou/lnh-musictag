// 新版本检查：启动后异步查询 GitHub Releases 最新 tag，与内置版本号比较，
// 通过 /api/version 暴露给前端做“有新版本”提示。查询失败（网络/限流）时静默
// 降级，不影响任何功能。结果缓存 30 分钟，避免频繁请求触发 GitHub 限流。
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type updateChecker struct {
	mu      sync.Mutex
	latest  string
	url     string
	checked time.Time
	repo    string
}

func newUpdateChecker(repo string) *updateChecker {
	return &updateChecker{repo: repo}
}

// Start launches the background check once at startup.
func (u *updateChecker) Start() {
	go u.check()
}

// check queries the GitHub Releases API (latest) and caches the result.
func (u *updateChecker) check() {
	const api = "https://api.github.com/repos/%s/releases/latest"
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.Replace(api, "%s", u.repo, 1), nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "LNH-MusicTag/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return // 无 release 或限流：静默
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return
	}
	if rel.TagName == "" {
		return
	}
	u.mu.Lock()
	u.latest = strings.TrimPrefix(rel.TagName, "v")
	u.url = rel.HTMLURL
	u.checked = time.Now()
	u.mu.Unlock()
}

// snapshot returns the cached comparison for the frontend.
func (u *updateChecker) snapshot() map[string]any {
	u.mu.Lock()
	defer u.mu.Unlock()
	return map[string]any{
		"current": AppVersion,
		"latest":  u.latest,
		"url":     u.url,
	}
}

// versionHandler reports current vs latest version.
func (u *updateChecker) versionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, u.snapshot())
}
