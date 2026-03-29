package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_New(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	require.NotNil(t, cb)
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	require.NotNil(t, config)
	assert.Equal(t, uint32(1), config.MaxRequests)
	assert.Equal(t, 60*time.Second, config.Interval)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, uint32(5), config.FailureThreshold)
}

func TestCircuitBreaker_SuccessExecution(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	err := cb.Execute(context.Background(), func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	_, successes, failures := cb.Counts()
	assert.Equal(t, uint32(1), successes)
	assert.Equal(t, uint32(0), failures)
}

func TestCircuitBreaker_FailureExecution(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:         1 * time.Second,
	})
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error { return errors.New("error") })
	}
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_OpenStateRejectsRequest(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1 * time.Second,
	})
	_ = cb.Execute(context.Background(), func() error { return errors.New("error") })
	err := cb.Execute(context.Background(), func() error { return nil })
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{FailureThreshold: 1})
	_ = cb.Execute(context.Background(), func() error { return errors.New("error") })
	assert.Equal(t, StateOpen, cb.State())
	cb.Reset()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	var called bool
	var from, to CircuitState
	cb := NewCircuitBreaker(&CircuitBreakerConfig{FailureThreshold: 1, Timeout: 1 * time.Second})
	cb.OnStateChange(func(f, t CircuitState) { called, from, to = true, f, t })
	_ = cb.Execute(context.Background(), func() error { return errors.New("error") })
	assert.True(t, called)
	assert.Equal(t, StateClosed, from)
	assert.Equal(t, StateOpen, to)
}

func TestCircuitBreaker_ErrorTypes(t *testing.T) {
	assert.Equal(t, "circuit breaker is open", ErrCircuitOpen.Error())
	assert.Equal(t, "too many requests", ErrTooManyRequests.Error())
}
