package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryStopsOnNonRetryableError(t *testing.T) {
	nonRetryable := errors.New("stop")
	attempts := 0

	err := Retry(context.Background(), RetryConfig{
		MaxAttempts:  4,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2,
		IsRetryable: func(err error) bool {
			return !errors.Is(err, nonRetryable)
		},
	}, func(context.Context) error {
		attempts++
		return nonRetryable
	})

	if !errors.Is(err, nonRetryable) {
		t.Fatalf("expected non-retryable error to be returned, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryEventuallySucceeds(t *testing.T) {
	attempts := 0

	err := Retry(context.Background(), RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2,
	}, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	err := Retry(ctx, RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   1,
		OnRetry: func(context.Context, RetryAttempt) {
			cancel()
		},
	}, func(context.Context) error {
		attempts++
		return errors.New("temporary")
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
