package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	coreconfig "task-processor/internal/core/config"
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

func TestDoRefreshesSDSAuthOnBadRequestAuthMessage(t *testing.T) {
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
					"message": "用户未登录",
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
}

func TestDoForcesLoginServiceRefreshOnStaleSDSAuth(t *testing.T) {
	t.Parallel()

	var forceLoginCalls int
	var authStateCalls int
	loginForced := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platforms/sds/login":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			forceLoginCalls++
			loginForced = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"message": "SDS 登录成功",
				"data": map[string]any{
					"tenant_id":   "tenant-1",
					"identifier":  "store-1",
					"merchant_id": 36811,
					"source":      "fresh_login",
				},
			})
		case "/api/platforms/sds/auth-state/tenant-1/store-1":
			authStateCalls++
			token := "stale-token"
			cookieValue := "stale-session"
			if loginForced {
				token = "fresh-token"
				cookieValue = "fresh-session"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"access_token": token,
					"merchant_id":  36811,
					"user_id":      30098709,
					"username":     "tester",
					"source":       "fresh_login",
					"cookies": []map[string]any{
						{
							"name":  "sid",
							"value": cookieValue,
							"path":  "/",
						},
					},
				},
			})
		case "/protected":
			cookie, cookieErr := r.Cookie("sid")
			if r.Header.Get("access-token") != "fresh-token" || cookieErr != nil || cookie.Value != "fresh-session" {
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
		LoginServiceBaseURL:    server.URL,
		LoginServiceTenantID:   "tenant-1",
		LoginServiceIdentifier: "store-1",
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
	if forceLoginCalls != 1 {
		t.Fatalf("expected 1 force login call, got %d", forceLoginCalls)
	}
	if authStateCalls != 1 {
		t.Fatalf("expected 1 auth-state call, got %d", authStateCalls)
	}
	state := c.AuthState()
	if state == nil || state.AccessToken != "fresh-token" {
		t.Fatalf("unexpected refreshed auth state: %+v", state)
	}
	if len(c.Cookies()) == 0 || c.Cookies()[0].Value != "fresh-session" {
		t.Fatalf("unexpected refreshed cookies: %+v", c.Cookies())
	}
}

func TestNewCallsLoginServiceWhenAuthStateMissing(t *testing.T) {
	t.Parallel()

	var loginCalls int
	var authStateCalls int
	loginCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platforms/sds/auth-state/tenant-1/store-1":
			authStateCalls++
			if !loginCalled {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"message": "当前没有可用的 SDS 登录态",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"access_token": "fresh-token",
					"merchant_id":  36811,
					"user_id":      30098709,
					"username":     "tester",
					"source":       "fresh_login",
					"cookies": []map[string]any{{
						"name":  "sid",
						"value": "fresh-session",
						"path":  "/",
					}},
				},
			})
		case "/api/platforms/sds/login":
			loginCalls++
			loginCalled = true
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode login request: %v", err)
			}
			if req["force_login"] != false {
				t.Fatalf("force_login = %v, want false for missing bootstrap state", req["force_login"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"message": "复用现有 SDS 登录态成功",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.AuthFile = filepath.Join(dir, "auth.json")
	cfg.CookieFile = filepath.Join(dir, "cookies.json")
	cfg.AuthBootstrap = AuthBootstrapConfig{
		LoginServiceBaseURL:    server.URL,
		LoginServiceTenantID:   "tenant-1",
		LoginServiceIdentifier: "store-1",
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("login calls = %d, want 1", loginCalls)
	}
	if authStateCalls != 2 {
		t.Fatalf("auth-state calls = %d, want 2", authStateCalls)
	}
	state := c.AuthState()
	if state == nil || state.AccessToken != "fresh-token" {
		t.Fatalf("auth state = %+v, want fresh token", state)
	}
	if len(c.Cookies()) == 0 || c.Cookies()[0].Value != "fresh-session" {
		t.Fatalf("cookies = %+v, want fresh session", c.Cookies())
	}
}

func TestNewCallsLoginServiceManualLoginWhenCredentialsConfigured(t *testing.T) {
	t.Parallel()

	var manualLoginCalls int
	var genericLoginCalls int
	var authStateCalls int
	loginCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/platforms/sds/auth-state/tenant-1/store-1":
			authStateCalls++
			if !loginCalled {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"message": "当前没有可用的 SDS 登录态",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"access_token": "fresh-token",
					"merchant_id":  36811,
					"user_id":      30098709,
					"username":     "tester",
					"source":       "fresh_login",
					"cookies": []map[string]any{{
						"name":  "sid",
						"value": "fresh-session",
						"path":  "/",
					}},
				},
			})
		case "/api/platforms/sds/manual-login":
			manualLoginCalls++
			loginCalled = true
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode manual login request: %v", err)
			}
			if req["merchant_name"] != "merchant" || req["username"] != "tester" || req["password"] != "secret" {
				t.Fatalf("unexpected manual login payload: %+v", req)
			}
			if req["identifier"] != "store-1" || req["tenant_id"] != "tenant-1" {
				t.Fatalf("unexpected manual login target: %+v", req)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"message": "SDS 登录成功",
			})
		case "/api/platforms/sds/login":
			genericLoginCalls++
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.AuthFile = filepath.Join(dir, "auth.json")
	cfg.CookieFile = filepath.Join(dir, "cookies.json")
	cfg.AuthBootstrap = AuthBootstrapConfig{
		LoginServiceBaseURL:    server.URL,
		LoginServiceTenantID:   "tenant-1",
		LoginServiceIdentifier: "store-1",
		LoginMerchantName:      "merchant",
		LoginUsername:          "tester",
		LoginPassword:          "secret",
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if manualLoginCalls != 1 {
		t.Fatalf("manual login calls = %d, want 1", manualLoginCalls)
	}
	if genericLoginCalls != 0 {
		t.Fatalf("generic login calls = %d, want 0", genericLoginCalls)
	}
	if authStateCalls != 2 {
		t.Fatalf("auth-state calls = %d, want 2", authStateCalls)
	}
	state := c.AuthState()
	if state == nil || state.AccessToken != "fresh-token" {
		t.Fatalf("auth state = %+v, want fresh token", state)
	}
}

func TestDoClearsStaleSDSAuthWhenNoRefreshSource(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/protected" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 20001,
			"msg": "用户未登录",
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	authFile := filepath.Join(dir, "auth.json")
	cookieFile := filepath.Join(dir, "cookies.json")
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.AuthFile = authFile
	cfg.CookieFile = cookieFile

	if err := NewAuthStateStore(authFile).Save(&AuthState{AccessToken: "stale-token", MerchantID: 36811}); err != nil {
		t.Fatalf("save auth state: %v", err)
	}
	if err := NewSessionStore(cookieFile).Save([]*http.Cookie{{Name: "sid", Value: "stale-session", Path: "/"}}); err != nil {
		t.Fatalf("save cookies: %v", err)
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = c.Do(context.Background(), http.MethodGet, "/protected", nil, nil, nil)
	if err == nil {
		t.Fatal("expected auth error")
	}
	if _, ok := err.(*AuthRequiredError); !ok {
		t.Fatalf("error = %T %v, want AuthRequiredError", err, err)
	}
	if state := c.AuthState(); state != nil {
		t.Fatalf("auth state = %+v, want nil", state)
	}
	if cookies := c.Cookies(); len(cookies) != 0 {
		t.Fatalf("cookies = %+v, want empty", cookies)
	}
	if loaded, loadErr := NewAuthStateStore(authFile).Load(); loadErr != nil || loaded != nil {
		t.Fatalf("persisted auth state = %+v, err=%v; want nil", loaded, loadErr)
	}
	if loaded, loadErr := NewSessionStore(cookieFile).Load(); loadErr != nil || len(loaded) != 0 {
		t.Fatalf("persisted cookies = %+v, err=%v; want empty", loaded, loadErr)
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

func TestAuthBootstrapHasSourceRequiresCompleteLoginServiceConfig(t *testing.T) {
	incomplete := AuthBootstrapConfig{LoginServiceBaseURL: "http://login:8000"}
	if incomplete.HasSource() {
		t.Fatal("incomplete login service config should not be treated as a refresh source")
	}

	complete := AuthBootstrapConfig{
		LoginServiceBaseURL:    "http://login:8000",
		LoginServiceTenantID:   "1",
		LoginServiceIdentifier: "869",
	}
	if !complete.HasSource() {
		t.Fatal("complete login service config should be treated as a refresh source")
	}
}

func TestNewManagementClientFromConfigRequiresCompleteConfig(t *testing.T) {
	if _, err := newManagementClientFromConfig(nil); err == nil {
		t.Fatal("expected nil config to fail")
	}

	if _, err := newManagementClientFromConfig(&coreconfig.ManagementConfig{
		BaseURL:  "https://api.example.test",
		ClientID: "client-id",
	}); err == nil {
		t.Fatal("expected incomplete management config to fail")
	}
}
