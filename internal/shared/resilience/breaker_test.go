package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		MaxRequests:      1,
		OpenTimeout:      25 * time.Millisecond,
		ReadyToTripAfter: 2,
	})

	for range 2 {
		_ = breaker.Execute(context.Background(), func(context.Context) error {
			return errors.New("boom")
		})
	}

	err := breaker.Execute(context.Background(), func(context.Context) error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if breaker.State() != StateOpen {
		t.Fatalf("breaker state = %v, want %v", breaker.State(), StateOpen)
	}
}

func TestCircuitBreakerHalfOpenThenClosesOnSuccess(t *testing.T) {
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "test",
		MaxRequests:      1,
		OpenTimeout:      100 * time.Millisecond,
		ReadyToTripAfter: 1,
	})

	_ = breaker.Execute(context.Background(), func(context.Context) error {
		return errors.New("boom")
	})

	if err := waitForCircuitProbe(t, breaker, 500*time.Millisecond); err != nil {
		t.Fatalf("half-open probe should succeed: %v", err)
	}
	if breaker.State() != StateClosed {
		t.Fatalf("breaker state = %v, want %v", breaker.State(), StateClosed)
	}
}

func waitForCircuitProbe(t *testing.T, breaker *CircuitBreaker, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := breaker.Execute(context.Background(), func(context.Context) error { return nil })
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrCircuitOpen) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}

	return context.DeadlineExceeded
}
