package sheinlogin

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestWaitAndConsumeVerifyCodeReturnsWhenCodeArrives(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct {
		code string
		ok   bool
		err  error
	}, 1)
	go func() {
		code, ok, err := store.WaitAndConsumeVerifyCode(ctx, 1, 2, 2*time.Second)
		done <- struct {
			code string
			ok   bool
			err  error
		}{code: code, ok: ok, err: err}
	}()

	time.Sleep(50 * time.Millisecond)
	if err := store.SubmitVerifyCode(context.Background(), 1, 2, "654321", 2*time.Minute); err != nil {
		t.Fatalf("submit verify code: %v", err)
	}

	result := <-done
	if result.err != nil {
		t.Fatalf("wait and consume verify code: %v", result.err)
	}
	if !result.ok || result.code != "654321" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestWaitAndConsumeVerifyCodeForAttemptDoesNotConsumeAnotherAttempt(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SubmitVerifyCodeForAttempt(context.Background(), 1, 2, "attempt-new", "654321", time.Minute); err != nil {
		t.Fatalf("submit attempt code: %v", err)
	}
	code, ok, err := store.WaitAndConsumeVerifyCodeForAttempt(context.Background(), 1, 2, "attempt-old", 20*time.Millisecond)
	if err != nil || ok || code != "" {
		t.Fatalf("old attempt consumed another code: code=%q ok=%v err=%v", code, ok, err)
	}
	code, ok, err = store.WaitAndConsumeVerifyCodeForAttempt(context.Background(), 1, 2, "attempt-new", time.Second)
	if err != nil || !ok || code != "654321" {
		t.Fatalf("new attempt did not receive its code: code=%q ok=%v err=%v", code, ok, err)
	}
}

func TestWaitAndConsumeVerifyCodeReturnsFalseOnTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })

	code, ok, err := store.WaitAndConsumeVerifyCode(context.Background(), 1, 2, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("wait and consume verify code: %v", err)
	}
	if ok || code != "" {
		t.Fatalf("unexpected result: code=%q ok=%v", code, ok)
	}
}

func TestWaitAndConsumeVerifyCodeHonorsContextCancellation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := newRedisStoreFromClient(client)
	t.Cleanup(func() { _ = store.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code, ok, err := store.WaitAndConsumeVerifyCode(ctx, 1, 2, time.Second)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if ok || code != "" {
		t.Fatalf("unexpected result: code=%q ok=%v err=%v", code, ok, err)
	}
}
