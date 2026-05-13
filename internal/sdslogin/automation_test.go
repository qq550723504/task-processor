package sdslogin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/playwright-community/playwright-go"
)

func TestIsLoginPageURL(t *testing.T) {
	if !isLoginPageURL("https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial") {
		t.Fatal("expected login page url to match")
	}
	if isLoginPageURL("https://www.sdsdiy.com/admin/material") {
		t.Fatal("expected non-login url to not match")
	}
}

func TestClassifyLoginFailureText(t *testing.T) {
	tests := []struct {
		name     string
		pageText string
		href     string
		want     string
	}{
		{
			name:     "credential error text",
			pageText: "账号或密码错误，请重新输入",
			want:     "SDS 登录失败，请检查账号密码",
		},
		{
			name:     "still on login page",
			pageText: "登录",
			href:     "https://www.sdsdiy.com/user/login",
			want:     "SDS 登录失败，请检查账号密码",
		},
		{
			name:     "other page state",
			pageText: "系统繁忙，请稍后再试",
			href:     "https://www.sdsdiy.com/error",
			want:     "SDS 登录未完成，请检查页面状态",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyLoginFailureFromSignals(tt.pageText, tt.href)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("classifyLoginFailure() = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCreateEphemeralProfileDir(t *testing.T) {
	root := t.TempDir()
	dir, cleanup, err := createEphemeralProfileDir(root, configuredAccount{TenantID: "1", Identifier: "869"})
	if err != nil {
		t.Fatalf("createEphemeralProfileDir() err = %v", err)
	}
	if filepath.Dir(dir) != filepath.Join(root, "attempts") {
		t.Fatalf("unexpected profile dir parent: %s", filepath.Dir(dir))
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("profile dir not created: %v", statErr)
	}
	cleanup()
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("profile dir should be removed, statErr=%v", statErr)
	}
}

func TestAcquireLoginPageRequiresContext(t *testing.T) {
	page, err := acquireLoginPage(nil)
	if err == nil || page != nil {
		t.Fatalf("acquireLoginPage(nil) = (%v, %v), want error", page, err)
	}
}

func TestHasUsableSessionMarkersAndLoginState(t *testing.T) {
	state := &pageLoginState{
		Token:      "token",
		MerchantID: 123,
	}
	if !hasUsableSessionMarkers(state) {
		t.Fatal("expected session markers to be usable")
	}
	if hasUsableLoginState(state) {
		t.Fatal("expected login state without browser cookies to be unusable")
	}
	state.BrowserState = map[string]any{
		"cookies": []playwright.OptionalCookie{{Name: "sid", Value: "cookie"}},
	}
	if !hasUsableLoginState(state) {
		t.Fatal("expected login state with browser cookies to be usable")
	}
}
