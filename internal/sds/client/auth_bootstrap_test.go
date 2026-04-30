package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestNewBootstrapsStaticSDSAuth(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.AuthFile = filepath.Join(dir, "auth.json")
	cfg.CookieFile = filepath.Join(dir, "cookies.json")
	cfg.AuthBootstrap = AuthBootstrapConfig{
		StaticAccessToken: "static-token",
		StaticMerchantID:  36811,
		StaticCookie:      "sid=abc123; theme=dark",
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	state := c.AuthState()
	if state == nil || state.AccessToken != "static-token" || state.MerchantID != 36811 {
		t.Fatalf("unexpected auth state: %+v", state)
	}
	if len(c.Cookies()) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(c.Cookies()))
	}
}

func TestDoRefreshesSDSAuthOnUnauthorizedBusinessCode(t *testing.T) {
	t.Parallel()

	var loginCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginCalls++
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "fresh-session", Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret": 0,
				"msg": "",
				"data": map[string]any{
					"access_token": "fresh-token",
					"merchant_id":  36811,
					"id":           1,
					"username":     "tester",
				},
			})
		case "/protected":
			_, err := r.Cookie("sid")
			if r.Header.Get("access-token") != "fresh-token" || err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ret": 20001,
					"msg": "用户未登录",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":  0,
				"msg":  "",
				"data": map[string]any{"ok": true},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.AuthFile = filepath.Join(dir, "auth.json")
	cfg.CookieFile = filepath.Join(dir, "cookies.json")
	cfg.AuthBootstrap = AuthBootstrapConfig{
		LoginUsername: "tester",
		LoginPassword: "secret",
	}

	stale := &AuthState{AccessToken: "stale-token", MerchantID: 36811}
	if err := NewAuthStateStore(cfg.AuthFile).Save(stale); err != nil {
		t.Fatalf("save auth state: %v", err)
	}
	if err := NewSessionStore(cfg.CookieFile).Save([]*http.Cookie{{Name: "sid", Value: "stale-session", Path: "/"}}); err != nil {
		t.Fatalf("save cookies: %v", err)
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var result map[string]any
	resp, err := c.Do(context.Background(), http.MethodGet, "/protected", nil, nil, &result)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp == nil || !resp.IsSuccessState() {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if loginCalls != 1 {
		t.Fatalf("expected 1 login call, got %d", loginCalls)
	}
	state := c.AuthState()
	if state == nil || state.AccessToken != "fresh-token" {
		t.Fatalf("unexpected refreshed auth state: %+v", state)
	}
	if len(c.Cookies()) == 0 || c.Cookies()[0].Value != "fresh-session" {
		t.Fatalf("unexpected refreshed cookies: %+v", c.Cookies())
	}
}

func TestNewBootstrapsSDSAuthFromLoginService(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/platforms/sds/auth-state/tenant-1/store-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Login-Shared-Key") != "shared-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"message": "invalid internal shared key",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"access_token": "login-service-token",
				"out_token":    "out-token",
				"merchant_id":  36811,
				"user_id":      30098709,
				"username":     "tester",
				"source":       "profile_reuse",
				"cookies": []map[string]any{
					{
						"name":    "sid",
						"value":   "cookie-from-login-service",
						"domain":  ".sdsdiy.com",
						"path":    "/",
						"expires": 1777544208,
					},
				},
			},
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.AuthFile = filepath.Join(dir, "auth.json")
	cfg.CookieFile = filepath.Join(dir, "cookies.json")
	cfg.AuthBootstrap = AuthBootstrapConfig{
		LoginServiceBaseURL:    server.URL,
		LoginServiceSharedKey:  "shared-key",
		LoginServiceTenantID:   "tenant-1",
		LoginServiceIdentifier: "store-1",
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	state := c.AuthState()
	if state == nil || state.AccessToken != "login-service-token" || state.MerchantID != 36811 {
		t.Fatalf("unexpected auth state: %+v", state)
	}
	if len(c.Cookies()) != 1 || c.Cookies()[0].Value != "cookie-from-login-service" {
		t.Fatalf("unexpected cookies: %+v", c.Cookies())
	}
	if c.Cookies()[0].Expires.IsZero() {
		t.Fatalf("expected cookie expires to be parsed: %+v", c.Cookies()[0])
	}
}
