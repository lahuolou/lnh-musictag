package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	sessionCookie = "lnh_session"
	sessionTTL    = 12 * time.Hour
)

// sessionStore holds active login sessions and the current admin credentials.
type sessionStore struct {
	mu         sync.Mutex
	sessions   map[string]time.Time // token -> expiry
	user       string
	pass       string
	configFile string // persists the admin password across restarts
}

func newSessionStore(user, pass, configFile string) *sessionStore {
	return &sessionStore{sessions: map[string]time.Time{}, user: user, pass: pass, configFile: configFile}
}

// resolveAdminPassword decides the effective admin password:
//  1. LNH_ADMIN_PASS env, if set, wins (fixed password).
//  2. otherwise a previously persisted password from the config file.
//  3. otherwise a freshly generated random password, persisted to the config
//     file so it stays stable across restarts. Returns generated=true when new.
func resolveAdminPassword(envPass, configFile string) (string, bool) {
	if envPass != "" {
		return envPass, false
	}
	if pw := loadAdminPassword(configFile); pw != "" {
		return pw, false
	}
	pw := randomPassword(12)
	_ = saveAdminPassword(configFile, pw) // best effort; startup log will surface it
	return pw, true
}

// loginHandler validates credentials and issues a session cookie.
func (s *sessionStore) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	okU := subtle.ConstantTimeCompare([]byte(req.User), []byte(s.user)) == 1
	okP := subtle.ConstantTimeCompare([]byte(req.Pass), []byte(s.pass)) == 1
	if !(okU && okP) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "账号或密码错误"})
		return
	}
	token := randomToken()
	exp := time.Now().Add(sessionTTL)
	s.mu.Lock()
	s.sessions[token] = exp
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "已登录", "user": s.user})
}

// logoutHandler clears the session.
func (s *sessionStore) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "已退出"})
}

// meHandler reports current auth status (200 authed / 401 not).
func (s *sessionStore) meHandler(w http.ResponseWriter, r *http.Request) {
	if s.valid(r) {
		writeJSON(w, http.StatusOK, map[string]string{"user": s.user})
		return
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
}

// changePasswordHandler (authenticated) validates the old password and updates
// the admin password, persisting it to the config file.
func (s *sessionStore) changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPass string `json:"oldPass"`
		NewPass string `json:"newPass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.OldPass), []byte(s.pass)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "旧密码错误"})
		return
	}
	if len(req.NewPass) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "新密码至少 6 位"})
		return
	}
	if s.configFile == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "未配置密码持久化，无法修改"})
		return
	}
	s.mu.Lock()
	s.pass = req.NewPass
	s.mu.Unlock()
	if err := saveAdminPassword(s.configFile, req.NewPass); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "密码已更新但持久化失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "密码已修改"})
}

// requireAuth guards an http.Handler; returns 401 when the session is invalid.
func (s *sessionStore) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.valid(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录或会话过期"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *sessionStore) valid(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, c.Value)
		return false
	}
	return true
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func randomPassword(n int) string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	rb := make([]byte, n)
	_, _ = rand.Read(rb)
	for i := range b {
		b[i] = chars[int(rb[i])%len(chars)]
	}
	return string(b)
}

type adminConfig struct {
	Password string `json:"password"`
}

func loadAdminPassword(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	var cfg adminConfig
	if json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	return cfg.Password
}

func saveAdminPassword(file, pw string) error {
	if dir := filepath.Dir(file); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	data, err := json.Marshal(adminConfig{Password: pw})
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o600)
}
