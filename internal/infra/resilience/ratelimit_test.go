package resilience

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterAllowHonorsBurst(t *testing.T) {
	limiter := NewRateLimiter(100, 2)

	if !limiter.Allow() {
		t.Fatal("first token should be allowed")
	}
	if !limiter.Allow() {
		t.Fatal("second token should be allowed")
	}
	if limiter.Allow() {
		t.Fatal("third token should be rejected once burst is exhausted")
	}
}

func TestRateLimiterWaitHonorsContextCancellation(t *testing.T) {
	limiter := NewRateLimiter(1, 1)
	if !limiter.Allow() {
		t.Fatal("initial token should be available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx)
	if err == nil {
		t.Fatal("Wait should fail when context expires before a token is available")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("Wait should stop promptly after context cancellation")
	}
}
