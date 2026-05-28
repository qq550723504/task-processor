package sdslogin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"task-processor/internal/core/config"
	sdsclient "task-processor/internal/sds/client"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(config.LoginServiceConfig{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "merchant",
		Username:     "user",
		Password:     "pass",
	}, config.RedisConfig{}, config.BrowserConfig{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	dir := t.TempDir()
	svc.authFile = filepath.Join(dir, "auth.json")
	svc.cookieFile = filepath.Join(dir, "cookies.json")
	svc.browserStateFile = filepath.Join(dir, "browser_state.json")
	svc.payloadFile = filepath.Join(dir, "login_state.json")
	svc.authStore = sdsclient.NewAuthStateStore(svc.authFile)
	svc.sessionStore = sdsclient.NewSessionStore(svc.cookieFile)
	return svc
}

func newRedisBackedTestService(t *testing.T) (*Service, *goredis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	svc := newTestService(t)
	svc.redisStore = newRedisStateStoreFromClient(client)
	t.Cleanup(func() { _ = client.Close() })
	return svc, client
}

func TestServiceStatusReadsPersistedPayload(t *testing.T) {
	svc := newTestService(t)
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
	if !status.HasAccessToken || status.Source != "fresh_login" || status.MerchantID != 12 || status.IssuedAt == nil {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestServiceLoadAuthStateReturnsPersistedAccessTokenWithoutCookies(t *testing.T) {
	svc := newTestService(t)
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
	if payload == nil || payload.AccessToken != "token" || len(payload.Cookies) != 0 {
		t.Fatalf("unexpected local auth payload: %+v", payload)
	}
}

func TestServiceLoadAuthStateUsesGlobalSharedState(t *testing.T) {
	svc := newTestService(t)
	if err := svc.persistPayload(&AuthPayload{
		TenantID:    "tenant-a",
		ShopID:      "store-a",
		Identifier:  "store-a",
		AccessToken: "token-a",
		Cookies:     []CookieRecord{{Name: "sid", Value: "a", Domain: ".sdsdiy.com", Path: "/"}},
	}); err != nil {
		t.Fatalf("persist payload: %v", err)
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
	if payloadB == nil || payloadB.AccessToken != "token-a" {
		t.Fatalf("unexpected payload B: %+v", payloadB)
	}
}

func TestServicePersistPayloadStoresMinimalRedisState(t *testing.T) {
	svc, client := newRedisBackedTestService(t)
	issuedAt := time.Now().UTC().Truncate(time.Second)
	if err := svc.persistPayload(&AuthPayload{
		TenantID:     "tenant-a",
		ShopID:       "store-a",
		Identifier:   "store-a",
		Username:     "user-a",
		MerchantName: "merchant-a",
		AccessToken:  "token-a",
		OutToken:     "out-a",
		MerchantID:   42,
		UserID:       84,
		Cookies:      []CookieRecord{{Name: "sid", Value: "cookie-a", Domain: ".sdsdiy.com", Path: "/"}},
		BrowserState: map[string]any{"cookies": []any{}, "origins": []any{"x"}},
		IssuedAt:     issuedAt,
		Source:       "fresh_login",
	}); err != nil {
		t.Fatalf("persist payload: %v", err)
	}

	raw, err := client.Get(context.Background(), sdsSharedAuthStateKey).Result()
	if err != nil {
		t.Fatalf("read redis payload: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("unmarshal redis payload: %v", err)
	}
	for _, forbidden := range []string{"tenant_id", "identifier", "shop_id", "username", "merchant_name", "browser_state", "issued_at", "source"} {
		if _, ok := stored[forbidden]; ok {
			t.Fatalf("unexpected extra field %q in redis payload: %v", forbidden, stored)
		}
	}
	if stored["access_token"] != "token-a" || stored["out_token"] != "out-a" {
		t.Fatalf("unexpected token fields in redis payload: %v", stored)
	}
}

func TestServiceLoadAuthStateUsesRedisSharedStateAcrossTenants(t *testing.T) {
	svc, _ := newRedisBackedTestService(t)
	if err := svc.persistPayload(&AuthPayload{
		TenantID:    "tenant-a",
		Identifier:  "store-a",
		ShopID:      "store-a",
		AccessToken: "token-a",
		OutToken:    "out-a",
		MerchantID:  12,
		UserID:      34,
		Cookies:     []CookieRecord{{Name: "sid", Value: "cookie-a", Domain: ".sdsdiy.com", Path: "/"}},
	}); err != nil {
		t.Fatalf("persist payload: %v", err)
	}

	payload, err := svc.LoadAuthState(context.Background(), "tenant-b", "store-b")
	if err != nil {
		t.Fatalf("load auth state: %v", err)
	}
	if payload == nil || payload.AccessToken != "token-a" || payload.MerchantID != 12 || len(payload.Cookies) != 0 {
		t.Fatalf("unexpected redis auth payload: %+v", payload)
	}
}

func TestServiceLoginReusesPersistedPayloadWithoutCookies(t *testing.T) {
	previous := runSDSBrowserLogin
	t.Cleanup(func() { runSDSBrowserLogin = previous })
	runSDSBrowserLogin = func(ctx context.Context, account configuredAccount, cfg browserRunConfig) (*AuthPayload, bool, error) {
		t.Fatal("runSDSBrowserLogin should not be called when persisted token is usable")
		return nil, false, nil
	}

	svc, _ := newRedisBackedTestService(t)
	if err := svc.persistPayload(&AuthPayload{
		TenantID:    "1",
		Identifier:  "869",
		ShopID:      "869",
		AccessToken: "token-a",
		MerchantID:  36811,
		UserID:      30098709,
	}); err != nil {
		t.Fatalf("persist payload: %v", err)
	}

	payload, err := svc.Login(withExplicitLoginTrigger(context.Background()), LoginRequest{})
	if err != nil {
		t.Fatalf("Login() err = %v", err)
	}
	if payload == nil || payload.AccessToken != "token-a" || payload.MerchantID != 36811 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestManualLoginFailureDoesNotOverwriteStoredAccount(t *testing.T) {
	previous := runSDSBrowserLogin
	t.Cleanup(func() { runSDSBrowserLogin = previous })
	runSDSBrowserLogin = func(ctx context.Context, account configuredAccount, cfg browserRunConfig) (*AuthPayload, bool, error) {
		return nil, false, fmt.Errorf("invalid credentials")
	}

	svc := newTestService(t)
	svc.account = configuredAccount{
		TenantID:     "1",
		Identifier:   "869",
		MerchantName: "good-merchant",
		Username:     "good-user",
		Password:     "good-pass",
	}

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

	svc := newTestService(t)

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

func TestTriggerLoginUsesCloakBrowserConfigWhenEnabled(t *testing.T) {
	previous := runSDSBrowserLogin
	t.Cleanup(func() { runSDSBrowserLogin = previous })
	t.Setenv("CLOAKBROWSER_BINARY_PATH", "C:/Users/test/.cloakbrowser/chrome.exe")

	var captured browserRunConfig
	runSDSBrowserLogin = func(ctx context.Context, account configuredAccount, cfg browserRunConfig) (*AuthPayload, bool, error) {
		captured = cfg
		return &AuthPayload{
			TenantID:    account.TenantID,
			Identifier:  account.Identifier,
			ShopID:      account.Identifier,
			AccessToken: "token",
			Cookies:     []CookieRecord{{Name: "sid", Value: "ok", Domain: ".sdsdiy.com", Path: "/"}},
		}, false, nil
	}

	svc, err := NewService(config.LoginServiceConfig{
		TenantID:            "1",
		Identifier:          "869",
		MerchantName:        "merchant",
		Username:            "user",
		Password:            "pass",
		CloakBrowserEnabled: true,
		CloakBrowserPath:    "D:/custom/cloak/chrome.exe",
		DefaultHeadless:     true,
	}, config.RedisConfig{}, config.BrowserConfig{BrowserPath: "D:/fallback/browser.exe"})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

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

	if !captured.UseCloakBrowser {
		t.Fatalf("expected UseCloakBrowser=true, got %+v", captured)
	}
	if captured.BrowserPath != "D:/custom/cloak/chrome.exe" {
		t.Fatalf("expected cloak browser path override, got %s", captured.BrowserPath)
	}
	if captured.Headless != true {
		t.Fatalf("expected headless=true, got %+v", captured)
	}
}

func TestLoginWithAccountRequiresExplicitTrigger(t *testing.T) {
	svc := newTestService(t)

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
