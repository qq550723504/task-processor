package sdslogin

import (
	"context"
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
		waiting  bool
		want     string
	}{
		{
			name:     "credential error text",
			pageText: "账号或密码错误，请重新输入",
			waiting:  false,
			want:     "SDS 登录失败，请检查账号密码",
		},
		{
			name:     "still on login page treated as verify wait",
			pageText: "登录",
			href:     "https://www.sdsdiy.com/user/login",
			waiting:  true,
			want:     "SDS 登录等待验证码或风控校验",
		},
		{
			name:     "explicit verify prompt",
			pageText: "请先完成验证码校验后再登录",
			href:     "https://www.sdsdiy.com/user/login",
			waiting:  true,
			want:     "SDS 登录等待验证码或风控校验",
		},
		{
			name:     "other page state",
			pageText: "系统繁忙，请稍后再试",
			href:     "https://www.sdsdiy.com/error",
			waiting:  false,
			want:     "SDS 登录未完成，请检查页面状态",
		},
		{
			name:     "non login unknown state",
			pageText: "接口处理失败",
			href:     "https://www.sdsdiy.com/error",
			waiting:  false,
			want:     "SDS 登录未完成，请检查页面状态",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waiting, err := classifyLoginFailureFromSignals(tt.pageText, tt.href)
			if waiting != tt.waiting {
				t.Fatalf("classifyLoginFailure() waiting = %t, want %t", waiting, tt.waiting)
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("classifyLoginFailure() err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveProfileDir(t *testing.T) {
	root := t.TempDir()
	dir, err := resolveProfileDir(root, configuredAccount{TenantID: "1", Identifier: "869"})
	if err != nil {
		t.Fatalf("resolveProfileDir() err = %v", err)
	}
	expected := filepath.Join(root, "1", "869")
	if dir != expected {
		t.Fatalf("unexpected profile dir: got %s want %s", dir, expected)
	}
}

func TestAcquireLoginPageRequiresContext(t *testing.T) {
	page, err := acquireLoginPage(context.Background(), nil)
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
	state.Href = "https://www.sdsdiy.com/admin/material"
	if !hasUsableLoginState(state) {
		t.Fatal("expected login state off the login page to be usable even without cookies")
	}
	state.Href = ""
	state.BrowserState = map[string]any{
		"cookies": []playwright.OptionalCookie{{Name: "sid", Value: "cookie"}},
	}
	if !hasUsableLoginState(state) {
		t.Fatal("expected login state with browser cookies to be usable")
	}
}
