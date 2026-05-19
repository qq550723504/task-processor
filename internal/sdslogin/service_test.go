package sdslogin

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"task-processor/internal/core/config"
	sdsclient "task-processor/internal/sds/client"
)

func TestServiceStatusReadsPersistedPayload(t *testing.T) {
	svc := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
	}, config.BrowserConfig{})
	dir := t.TempDir()
	svc.authFile = filepath.Join(dir, "auth.json")
	svc.cookieFile = filepath.Join(dir, "cookies.json")
	svc.browserStateFile = filepath.Join(dir, "browser_state.json")
	svc.payloadFile = filepath.Join(dir, "login_state.json")
	svc.authStore = sdsclient.NewAuthStateStore(svc.authFile)
	svc.sessionStore = sdsclient.NewSessionStore(svc.cookieFile)

	issuedAt := time.Now().UTC().Truncate(time.Second)
	if err := svc.persistPayload(&AuthPayload{
		TenantID:     "1",
		ShopID:       "869",
		Identifier:   "869",
		Username:     "user",
		MerchantName: "merchant",
		AccessToken:  "token",
		MerchantID:   12,
		Cookies:      []CookieRecord{{Name: "sid", Value: "ok", Domain: ".sdsdiy.com", Path: "/"}},
		BrowserState: map[string]any{"cookies": []any{}},
		IssuedAt:     issuedAt,
		Source:       "fresh_login",
	}); err != nil {
		t.Fatalf("persist payload: %v", err)
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.HasCookie || !status.HasAccessToken || status.Source != "fresh_login" || status.MerchantID != 12 || status.IssuedAt == nil {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestServiceLoadAuthStateReturnsPersistedCookies(t *testing.T) {
	svc := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
	}, config.BrowserConfig{})
	dir := t.TempDir()
	svc.authFile = filepath.Join(dir, "auth.json")
	svc.cookieFile = filepath.Join(dir, "cookies.json")
	svc.browserStateFile = filepath.Join(dir, "browser_state.json")
	svc.payloadFile = filepath.Join(dir, "login_state.json")
	svc.authStore = sdsclient.NewAuthStateStore(svc.authFile)
	svc.sessionStore = sdsclient.NewSessionStore(svc.cookieFile)

	if err := svc.persistPayload(&AuthPayload{
		TenantID:    "1",
		ShopID:      "869",
		Identifier:  "869",
		Username:    "user",
		AccessToken: "token",
		Cookies:     []CookieRecord{{Name: "sid", Value: "ok", Domain: ".sdsdiy.com", Path: "/"}},
		Source:      "fresh_login",
	}); err != nil {
		t.Fatalf("persist payload: %v", err)
	}
	payload, err := svc.LoadAuthState(context.Background(), "1", "869")
	if err != nil {
		t.Fatalf("load auth state: %v", err)
	}
	if payload == nil || payload.AccessToken != "token" || len(payload.Cookies) != 1 {
		t.Fatalf("unexpected local auth payload: %+v", payload)
	}
}

func TestServiceLoadAuthStateIsScopedByTenantAndIdentifier(t *testing.T) {
	svc := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
	}, config.BrowserConfig{})
	dir := t.TempDir()
	svc.authFile = filepath.Join(dir, "auth.json")
	svc.cookieFile = filepath.Join(dir, "cookies.json")
	svc.browserStateFile = filepath.Join(dir, "browser_state.json")
	svc.payloadFile = filepath.Join(dir, "login_state.json")
	svc.authStore = sdsclient.NewAuthStateStore(svc.authFile)
	svc.sessionStore = sdsclient.NewSessionStore(svc.cookieFile)

	if err := svc.persistPayload(&AuthPayload{
		TenantID:    "tenant-a",
		ShopID:      "store-a",
		Identifier:  "store-a",
		AccessToken: "token-a",
		Cookies:     []CookieRecord{{Name: "sid", Value: "a", Domain: ".sdsdiy.com", Path: "/"}},
	}); err != nil {
		t.Fatalf("persist payload A: %v", err)
	}
	if err := svc.persistPayload(&AuthPayload{
		TenantID:    "tenant-b",
		ShopID:      "store-b",
		Identifier:  "store-b",
		AccessToken: "token-b",
		Cookies:     []CookieRecord{{Name: "sid", Value: "b", Domain: ".sdsdiy.com", Path: "/"}},
	}); err != nil {
		t.Fatalf("persist payload B: %v", err)
	}

	payloadA, err := svc.LoadAuthState(context.Background(), "tenant-a", "store-a")
	if err != nil {
		t.Fatalf("load auth state A: %v", err)
	}
	if payloadA == nil || payloadA.AccessToken != "token-a" {
		t.Fatalf("unexpected payload A: %+v", payloadA)
	}

	payloadB, err := svc.LoadAuthState(context.Background(), "tenant-b", "store-b")
	if err != nil {
		t.Fatalf("load auth state B: %v", err)
	}
	if payloadB == nil || payloadB.AccessToken != "token-b" {
		t.Fatalf("unexpected payload B: %+v", payloadB)
	}

	payloadMissing, err := svc.LoadAuthState(context.Background(), "tenant-a", "store-b")
	if err != nil {
		t.Fatalf("load auth state missing: %v", err)
	}
	if payloadMissing != nil {
		t.Fatalf("expected nil payload for mismatched account, got %+v", payloadMissing)
	}
}

func TestManualLoginFailureDoesNotOverwriteStoredAccount(t *testing.T) {
	previous := runSDSBrowserLogin
	t.Cleanup(func() { runSDSBrowserLogin = previous })
	runSDSBrowserLogin = func(ctx context.Context, account configuredAccount, cfg browserRunConfig) (*AuthPayload, bool, error) {
		return nil, false, fmt.Errorf("invalid credentials")
	}

	svc := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "good-merchant",
		Username:     "good-user",
		Password:     "good-pass",
	}, config.BrowserConfig{})
	dir := t.TempDir()
	svc.authFile = filepath.Join(dir, "auth.json")
	svc.cookieFile = filepath.Join(dir, "cookies.json")
	svc.browserStateFile = filepath.Join(dir, "browser_state.json")
	svc.payloadFile = filepath.Join(dir, "login_state.json")
	svc.authStore = sdsclient.NewAuthStateStore(svc.authFile)
	svc.sessionStore = sdsclient.NewSessionStore(svc.cookieFile)

	_, err := svc.ManualLogin(context.Background(), ManualLoginRequest{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "bad-merchant",
		Username:     "bad-user",
		Password:     "bad-pass",
		ForceLogin:   true,
	})
	if err == nil {
		t.Fatal("expected manual login to fail")
	}

	status, statusErr := svc.Status(context.Background())
	if statusErr != nil {
		t.Fatalf("status: %v", statusErr)
	}
	if status.MerchantName != "good-merchant" || status.Username != "good-user" {
		t.Fatalf("stored account was overwritten after failed login: %+v", status)
	}
}

func TestTriggerLoginLaunchesBrowserLogin(t *testing.T) {
	previous := runSDSBrowserLogin
	t.Cleanup(func() { runSDSBrowserLogin = previous })
	callCount := 0
	runSDSBrowserLogin = func(ctx context.Context, account configuredAccount, cfg browserRunConfig) (*AuthPayload, bool, error) {
		callCount++
		return &AuthPayload{
			TenantID:    account.TenantID,
			Identifier:  account.Identifier,
			ShopID:      account.Identifier,
			AccessToken: "token",
			Cookies:     []CookieRecord{{Name: "sid", Value: "ok", Domain: ".sdsdiy.com", Path: "/"}},
		}, false, nil
	}

	svc := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
		Password:     "pass",
	}, config.BrowserConfig{})

	if err := svc.TriggerLogin(context.Background(), sdsclient.LocalLoginRequest{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
		Password:     "pass",
		ForceLogin:   true,
	}); err != nil {
		t.Fatalf("TriggerLogin() err = %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected TriggerLogin to launch browser once, got %d calls", callCount)
	}
}

func TestLoginWithAccountRequiresExplicitTrigger(t *testing.T) {
	svc := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
		Password:     "pass",
	}, config.BrowserConfig{})

	_, err := svc.loginWithAccount(context.Background(), configuredAccount{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
		Password:     "pass",
	}, LoginRequest{ForceLogin: true})
	if err == nil || err.Error() != "SDS 登录仅允许通过显式登录入口触发" {
		t.Fatalf("loginWithAccount() err = %v", err)
	}
}
