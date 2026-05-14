package sheinlogin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisStoreCookieAndVerifyCodeLifecycle(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	payload := map[string]any{"cookies": []map[string]any{{"name": "sid", "value": "123"}}}
	if err := store.SaveCookieState(ctx, 1, 2, payload, time.Hour); err != nil {
		t.Fatalf("save cookie state: %v", err)
	}
	if has, err := store.HasCookie(ctx, 1, 2); err != nil || !has {
		t.Fatalf("expected cookie to exist, has=%v err=%v", has, err)
	}
	if ttl, ok, err := store.CookieTTL(ctx, 1, 2); err != nil || !ok || ttl <= 0 {
		t.Fatalf("expected cookie ttl, ttl=%v ok=%v err=%v", ttl, ok, err)
	}

	if err := store.SubmitVerifyCode(ctx, 1, 2, "654321", 2*time.Minute); err != nil {
		t.Fatalf("submit verify code: %v", err)
	}
	if waiting, err := store.IsWaitingVerifyCode(ctx, 1, 2); err != nil || !waiting {
		t.Fatalf("expected verify wait, waiting=%v err=%v", waiting, err)
	}
	if code, ok, err := store.ConsumeVerifyCode(ctx, 1, 2); err != nil || !ok || code != "654321" {
		t.Fatalf("unexpected consumed code: code=%q ok=%v err=%v", code, ok, err)
	}
	if waiting, err := store.IsWaitingVerifyCode(ctx, 1, 2); err != nil || waiting {
		t.Fatalf("expected verify wait cleared, waiting=%v err=%v", waiting, err)
	}

	if err := store.RecordLastLoginTime(ctx, 1, 2, time.Unix(1700000000, 0)); err != nil {
		t.Fatalf("record last login: %v", err)
	}
	last, err := store.LastLoginTime(ctx, 1, 2)
	if err != nil || last == nil || last.Unix() != 1700000000 {
		t.Fatalf("unexpected last login: last=%v err=%v", last, err)
	}

	summary := &FailureSummary{
		ErrorCode:    "REQUEST_FAILED",
		ErrorMessage: "请求失败",
		ArtifactPath: "D:\\tmp\\artifact",
	}
	if err := store.RecordLastFailure(ctx, 1, 2, summary, time.Hour); err != nil {
		t.Fatalf("record last failure: %v", err)
	}
	failure, err := store.LastFailure(ctx, 1, 2)
	if err != nil || failure == nil || failure.ErrorCode != "REQUEST_FAILED" || failure.ArtifactPath != summary.ArtifactPath {
		t.Fatalf("unexpected last failure: failure=%+v err=%v", failure, err)
	}
	if err := store.ClearLastFailure(ctx, 1, 2); err != nil {
		t.Fatalf("clear last failure: %v", err)
	}
	failure, err = store.LastFailure(ctx, 1, 2)
	if err != nil || failure != nil {
		t.Fatalf("expected cleared last failure: failure=%+v err=%v", failure, err)
	}
}

func TestSaveCookieStateStripsNonCookieBrowserState(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	payload := map[string]any{
		"cookies": []map[string]any{{"name": "sid", "value": "123"}},
		"origins": []map[string]any{{"origin": "https://sellerhub.shein.com"}},
	}
	if err := store.SaveCookieState(ctx, 1, 2, payload, time.Hour); err != nil {
		t.Fatalf("save cookie state: %v", err)
	}

	raw, err := client.Get(ctx, cookieKey(1, 2)).Result()
	if err != nil {
		t.Fatalf("load saved payload: %v", err)
	}

	var saved map[string]any
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		t.Fatalf("unmarshal saved payload: %v", err)
	}
	if _, ok := saved["origins"]; ok {
		t.Fatalf("expected origins to be stripped, payload=%v", saved)
	}
	cookies, ok := saved["cookies"].([]any)
	if !ok || len(cookies) != 1 {
		t.Fatalf("expected cookies to be preserved, payload=%v", saved)
	}
}
