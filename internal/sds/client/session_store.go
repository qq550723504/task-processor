package client

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/pkg/jsonx"
)

// PersistedCookie 用于本地持久化 Cookie。
type PersistedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HttpOnly bool      `json:"httpOnly,omitempty"`
}

// SessionStore 负责 SDS Cookie 的本地读写。
type SessionStore struct {
	filePath string
}

// NewSessionStore 创建本地会话存储。
func NewSessionStore(filePath string) *SessionStore {
	return &SessionStore{filePath: filePath}
}

// Load 读取持久化 Cookie。
func (s *SessionStore) Load() ([]*http.Cookie, error) {
	if s.filePath == "" {
		return nil, fmt.Errorf("cookie file path is empty")
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cookie file: %w", err)
	}

	var persisted []PersistedCookie
	if err := jsonx.UnmarshalBytes(data, &persisted, "parse persisted cookies"); err != nil {
		return nil, err
	}

	cookies := make([]*http.Cookie, 0, len(persisted))
	for _, item := range persisted {
		cookies = append(cookies, item.toHTTPCookie())
	}

	return cookies, nil
}

// Save 保存 Cookie 到本地。
func (s *SessionStore) Save(cookies []*http.Cookie) error {
	if s.filePath == "" {
		return fmt.Errorf("cookie file path is empty")
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create cookie dir: %w", err)
	}

	persisted := make([]PersistedCookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		persisted = append(persisted, persistedCookieFromHTTP(cookie))
	}

	data, err := jsonx.MarshalPretty(persisted)
	if err != nil {
		return fmt.Errorf("marshal cookies: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("write cookie file: %w", err)
	}

	return nil
}

// Clear 删除本地 Cookie 状态。
func (s *SessionStore) Clear() error {
	if s.filePath == "" {
		return fmt.Errorf("cookie file path is empty")
	}
	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cookie file: %w", err)
	}
	return nil
}

// ParseCookieHeader 将 `a=1; b=2` 形式的 Cookie Header 转成 http.Cookie。
func ParseCookieHeader(raw, domain string) []*http.Cookie {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	pairs := strings.Split(raw, ";")
	cookies := make([]*http.Cookie, 0, len(pairs))

	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if name == "" {
			continue
		}

		cookies = append(cookies, &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   "/",
		})
	}

	return cookies
}

func persistedCookieFromHTTP(cookie *http.Cookie) PersistedCookie {
	item := PersistedCookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Domain:   cookie.Domain,
		Path:     cookie.Path,
		Secure:   cookie.Secure,
		HttpOnly: cookie.HttpOnly,
	}

	if !cookie.Expires.IsZero() {
		item.Expires = cookie.Expires
	}

	if item.Path == "" {
		item.Path = "/"
	}

	return item
}

func (p PersistedCookie) toHTTPCookie() *http.Cookie {
	cookie := &http.Cookie{
		Name:     p.Name,
		Value:    p.Value,
		Domain:   p.Domain,
		Path:     p.Path,
		Secure:   p.Secure,
		HttpOnly: p.HttpOnly,
	}

	if cookie.Path == "" {
		cookie.Path = "/"
	}

	if !p.Expires.IsZero() {
		cookie.Expires = p.Expires
	}

	return cookie
}
