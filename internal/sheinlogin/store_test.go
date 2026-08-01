package sheinlogin

import (
	"context"
	"encoding/json"
	"sync"
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

func TestLoadCookieStateReturnsStoredPayload(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	payload := map[string]any{"cookies": []map[string]any{{"name": "sid", "value": "123"}}}
	if err := store.SaveCookieState(ctx, 1, 2, payload, time.Hour); err != nil {
		t.Fatalf("save cookie state: %v", err)
	}

	raw, ok, err := store.LoadCookieState(ctx, 1, 2)
	if err != nil {
		t.Fatalf("load cookie state: %v", err)
	}
	if !ok {
		t.Fatal("expected stored cookie state to exist")
	}
	if raw == "" {
		t.Fatal("expected non-empty stored cookie state payload")
	}
}

func TestEnqueueLoginAttemptIsCredentialFreeAndDeduplicated(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })

	headless := true
	first, created, err := store.EnqueueLoginAttempt(context.Background(), 1, 2, LoginRequest{ForceLogin: true, Headless: &headless})
	if err != nil {
		t.Fatalf("enqueue first attempt: %v", err)
	}
	if !created || first == nil || first.Status != LoginAttemptQueued {
		t.Fatalf("unexpected first attempt: created=%v attempt=%+v", created, first)
	}
	second, created, err := store.EnqueueLoginAttempt(context.Background(), 1, 2, LoginRequest{})
	if err != nil {
		t.Fatalf("enqueue duplicate attempt: %v", err)
	}
	if created || second == nil || second.ID != first.ID {
		t.Fatalf("duplicate enqueue should return the active attempt: created=%v first=%+v second=%+v", created, first, second)
	}

	raw, err := client.Get(context.Background(), loginAttemptKey(first.ID)).Result()
	if err != nil {
		t.Fatalf("load attempt payload: %v", err)
	}
	if contains := string(raw); contains == "" || contains == "password" || contains == "pwd" {
		t.Fatalf("attempt payload contains credential data: %s", raw)
	}
}

func TestEnqueueLoginAttemptIsAtomicForConcurrentRequests(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })

	const callers = 16
	results := make(chan struct {
		attempt *LoginAttempt
		created bool
		err     error
	}, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt, created, err := store.EnqueueLoginAttempt(context.Background(), 1, 2, LoginRequest{ForceLogin: true})
			results <- struct {
				attempt *LoginAttempt
				created bool
				err     error
			}{attempt, created, err}
		}()
	}
	wg.Wait()
	close(results)

	var id string
	createdCount := 0
	for result := range results {
		if result.err != nil || result.attempt == nil {
			t.Fatalf("enqueue result: attempt=%+v err=%v", result.attempt, result.err)
		}
		if id == "" {
			id = result.attempt.ID
		}
		if result.attempt.ID != id {
			t.Fatalf("expected one attempt, got %q and %q", id, result.attempt.ID)
		}
		if result.created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created attempts = %d, want 1", createdCount)
	}
}
