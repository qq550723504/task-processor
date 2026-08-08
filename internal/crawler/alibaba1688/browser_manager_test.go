package alibaba1688

import (
	"task-processor/internal/core/config"
	"testing"
)

func TestResolveAlibaba1688UserDataDirUsesConfiguredValue(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Browser.UserDataDir = "./.local/tmp/browser-profiles/custom-1688"

	got := resolveAlibaba1688UserDataDir(cfg)

	if got != "./.local/tmp/browser-profiles/custom-1688" {
		t.Fatalf("expected configured user data dir, got %q", got)
	}
}

func TestResolveAlibaba1688UserDataDirUsesSharedDefault(t *testing.T) {
	cfg := config.NewDefaultConfig()

	got := resolveAlibaba1688UserDataDir(cfg)

	if got == "" {
		t.Fatal("expected non-empty default user data dir")
	}
}

func TestAlibaba1688BrowserRuntimeConfigUsesAccountProfileWithoutMutatingGlobalConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Browser.Headless = false
	cfg.Browser.BrowserPath = "C:/browsers/chrome.exe"
	cfg.Browser.UserDataDir = "C:/global-profile"
	cfg.Browser.ProxyServer = "http://global-proxy:8080"
	cfg.Browser.ViewportWidth = 1600
	cfg.Browser.ViewportHeight = 1000

	profile := &AccountProfile{
		ID:          3001,
		TenantID:    101,
		ProfileDir:  "C:/account-profiles/101/3001",
		ProxyServer: "http://account-proxy:8080",
	}

	runtimeConfig := newAlibaba1688BrowserRuntimeConfig(cfg, profile)

	if runtimeConfig.userDataDir != profile.ProfileDir {
		t.Fatalf("user data dir = %q, want %q", runtimeConfig.userDataDir, profile.ProfileDir)
	}
	if runtimeConfig.browser.ProxyServer != profile.ProxyServer {
		t.Fatalf("proxy = %q, want account proxy", runtimeConfig.browser.ProxyServer)
	}
	if runtimeConfig.browser.BrowserPath != cfg.Browser.BrowserPath ||
		runtimeConfig.browser.Headless != cfg.Browser.Headless ||
		runtimeConfig.browser.ViewportWidth != cfg.Browser.ViewportWidth ||
		runtimeConfig.browser.ViewportHeight != cfg.Browser.ViewportHeight {
		t.Fatal("account profile changed unrelated browser settings")
	}
	if cfg.Browser.UserDataDir != "C:/global-profile" || cfg.Browser.ProxyServer != "http://global-proxy:8080" {
		t.Fatal("account profile mutated the process-wide browser configuration")
	}
}

func TestAlibaba1688BrowserRuntimeConfigUsesGlobalFallbackWithoutAccount(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Browser.UserDataDir = "C:/global-profile"
	cfg.Browser.ProxyServer = "http://global-proxy:8080"

	runtimeConfig := newAlibaba1688BrowserRuntimeConfig(cfg, nil)

	if runtimeConfig.userDataDir != cfg.Browser.UserDataDir {
		t.Fatalf("user data dir = %q, want global fallback %q", runtimeConfig.userDataDir, cfg.Browser.UserDataDir)
	}
	if runtimeConfig.browser.ProxyServer != cfg.Browser.ProxyServer {
		t.Fatalf("proxy = %q, want global fallback %q", runtimeConfig.browser.ProxyServer, cfg.Browser.ProxyServer)
	}
}
