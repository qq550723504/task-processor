// Package resilience provides unit tests for circuit breaker
package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Circuit Breaker Tests ====================

// TestCircuitBreaker_New tests creating a new circuit breaker
func TestCircuitBreaker_New(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	require.NotNil(t, cb)
	
	assert.Equal(t, StateClosed, cb.State(), "initial state should be closed")
}

// TestCircuitBreaker_DefaultConfig tests default configuration
func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	require.NotNil(t, config)
	
	assert.Equal(t, uint32(1), config.MaxRequests)
	assert.Equal(t, 60*time.Second, config.Interval)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, uint32(5), config.FailureThreshold)
	assert.Equal(t, 0.5, config.FailureRateThresh)
	assert.Equal(t, uint32(10), config.MinRequests)
}

// TestCircuitBreaker_SuccessExecution tests successful execution
func TestCircuitBreaker_SuccessExecution(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	
	requests, successes, failures := cb.Counts()
	assert.Equal(t, uint32(1), requests)
	assert.Equal(t, uint32(1), successes)
	assert.Equal(t, uint32(0), failures)
}

// TestCircuitBreaker_FailureExecution tests failure execution
func TestCircuitBreaker_FailureExecution(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          1 * time.Second,
	})
	
	// Execute 3 failures, should open circuit
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return errors.New("error")
		})
	}
	
	assert.Equal(t, StateOpen, cb.State(), "should open circuit after 3 consecutive failures")
}

// TestCircuitBreaker_FailureRateThreshold tests failure rate threshold
func TestCircuitBreaker_FailureRateThreshold(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MinRequests:      10,
		FailureRateThresh: 0.5, // 50% failure rate
		Timeout:          1 * time.Second,
	})
	
	// 10 requests, 6 failures (60%), should open
	for i := 0; i < 10; i++ {
		err := errors.New("error")
		if i < 6 {
			_ = cb.Execute(context.Background(), func() error { return err })
		} else {
			_ = cb.Execute(context.Background(), func() error { return nil })
		}
	}
	
	assert.Equal(t, StateOpen, cb.State(), "60% failure rate should open circuit")
}

// TestCircuitBreaker_OpenStateRejectsRequest tests open state rejects requests
func TestCircuitBreaker_OpenStateRejectsRequest(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:         1 * time.Second,
	})
	
	// First failure opens circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	assert.Equal(t, StateOpen, cb.State())
	
	// Should reject when circuit is open
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

// TestCircuitBreaker_HalfOpenState tests half-open state
func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          50 * time.Millisecond, // short timeout
	})
	
	// Open circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	assert.Equal(t, StateOpen, cb.State())
	
	// Wait for timeout
	time.Sleep(60 * time.Millisecond)
	
	// Should allow one request in half-open state
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	
	assert.NoError(t, err, "half-open state should allow request")
}

// TestCircuitBreaker_RecoveryAfterHalfOpen tests recovery after half-open success
func TestCircuitBreaker_RecoveryAfterHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})
	
	// Open circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	
	// Success in half-open should recover to closed
	_ = cb.Execute(context.Background(), func() error {
		return nil
	})
	
	assert.Equal(t, StateClosed, cb.State(), "should recover to closed after success")
}

// TestCircuitBreaker_Reset tests reset functionality
func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
	})
	
	// Open circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	assert.Equal(t, StateOpen, cb.State())
	
	// Reset
	cb.Reset()
	
	assert.Equal(t, StateClosed, cb.State())
	requests, successes, failures := cb.Counts()
	assert.Equal(t, uint32(0), requests)
	assert.Equal(t, uint32(0), successes)
	assert.Equal(t, uint32(0), failures)
}

// TestCircuitBreaker_StateChangeCallback tests state change callback
func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	callbackCalled := false
	var oldState, newState CircuitState
	
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1 * time.Second,
	})
	
	cb.OnStateChange(func(from, to CircuitState) {
		callbackCalled = true
		oldState = from
		newState = to
	})
	
	// Open circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	assert.True(t, callbackCalled)
	assert.Equal(t, StateClosed, oldState)
	assert.Equal(t, StateOpen, newState)
}

// TestCircuitBreaker_ConcurrentAccess tests concurrent access
func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 10,
	})
	
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			_ = cb.Execute(context.Background(), func() error {
				return nil
			})
			done <- true
		}()
	}
	
	// Wait for all requests
	for i := 0; i < 100; i++ {
		<-done
	}
	
	requests, successes, failures := cb.Counts()
	assert.Equal(t, uint32(100), requests)
	assert.Equal(t, uint32(100), successes)
	assert.Equal(t, uint32(0), failures)
}

// TestCircuitBreaker_AllFailuresThenSuccess tests all failures then success recovers
func TestCircuitBreaker_AllFailuresThenSuccess(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 5,
		Timeout:         100 * time.Millisecond,
	})
	
	// 5 failures
	for i := 0; i < 5; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return errors.New("error")
		})
	}
	
	assert.Equal(t, StateOpen, cb.State())
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Half-open: first request fails
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	// Should go back to open
	assert.Equal(t, StateOpen, cb.State())
}

// TestCircuitBreaker_MaxRequestsInHalfOpen tests max requests limit in half-open state
func TestCircuitBreaker_MaxRequestsInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          50 * time.Millisecond,
		MaxRequests:      1, // Only 1 request allowed in half-open
	})
	
	// Open circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("error")
	})
	
	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	
	// First request in half-open should succeed
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
	
	// Second request should be rejected
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many requests")
}

// TestCircuitBreaker_ErrorTypes tests error constants
func TestCircuitBreaker_ErrorTypes(t *testing.T) {
	assert.Equal(t, "circuit breaker is open", ErrCircuitOpen.Error())
	assert.Equal(t, "too many requests", ErrTooManyRequests.Error())
}
