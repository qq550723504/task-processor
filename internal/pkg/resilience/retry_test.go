// Package resilience provides unit tests for retry mechanism
package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Retry Tests ====================

// TestRetry_SuccessFirstAttempt tests first attempt success
func TestRetry_SuccessFirstAttempt(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return nil
	}

	err := Retry(context.Background(), nil, fn)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "should call once")
}

// TestRetry_SuccessAfterRetries tests success after multiple retries
func TestRetry_SuccessAfterRetries(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxJitter:    1 * time.Millisecond,
	}

	err := Retry(context.Background(), config, fn)

	require.NoError(t, err)
	assert.Equal(t, 3, callCount, "should retry 3 times")
}

// TestRetry_MaxAttemptsExceeded tests max retry attempts exceeded
func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return errors.New("persistent error")
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxJitter:    0,
	}

	err := Retry(context.Background(), config, fn)

	assert.Error(t, err)
	assert.Equal(t, 3, callCount, "should retry 3 times")
	assert.Contains(t, err.Error(), "max retry attempts")
}

// TestRetry_NonRetryableError tests non-retryable error
func TestRetry_NonRetryableError(t *testing.T) {
	callCount := 0
	retryableErr := errors.New("retryable error")
	nonRetryableErr := errors.New("non-retryable error")

	fn := func(ctx context.Context) error {
		callCount++
		return nonRetryableErr
	}

	config := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:      2.0,
		MaxJitter:       0,
		RetryableErrors: []error{retryableErr},
	}

	err := Retry(context.Background(), config, fn)

	assert.Error(t, err)
	assert.Equal(t, 1, callCount, "non-retryable error should return immediately")
	assert.Contains(t, err.Error(), "non-retryable error")
}

// TestRetry_ContextCancellation tests context cancellation
func TestRetry_ContextCancellation(t *testing.T) {
	fn := func(ctx context.Context) error {
		return errors.New("temporary error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	config := &RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxJitter:    0,
	}

	err := Retry(ctx, config, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry canceled")
}

// TestRetry_ExponentialBackoff tests exponential backoff timing
func TestRetry_ExponentialBackoff(t *testing.T) {
	callTimes := []time.Time{}

	fn := func(ctx context.Context) error {
		callTimes = append(callTimes, time.Now())
		if len(callTimes) < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxJitter:    0,
	}

	err := Retry(context.Background(), config, fn)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(callTimes), 3)

	// Second call should be at least 100ms after first
	delay1 := callTimes[1].Sub(callTimes[0])
	assert.GreaterOrEqual(t, delay1.Milliseconds(), int64(90), "first delay should be ~100ms")

	// Third call should be at least 200ms after second (exponential)
	delay2 := callTimes[2].Sub(callTimes[1])
	assert.GreaterOrEqual(t, delay2.Milliseconds(), int64(190), "second delay should be ~200ms")
}

// TestRetry_WithCustomRetryableErrors tests custom retryable errors
func TestRetry_WithCustomRetryableErrors(t *testing.T) {
	callCount := 0
	errTemporary := errors.New("temporary error")

	fn := func(ctx context.Context) error {
		callCount++
		return errTemporary
	}

	config := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:      2.0,
		MaxJitter:       0,
		RetryableErrors: []error{errTemporary},
	}

	err := Retry(context.Background(), config, fn)

	// errTemporary is in RetryableErrors, should retry
	assert.Error(t, err)
	assert.Equal(t, 3, callCount)
	assert.Contains(t, err.Error(), "max retry attempts")
}

// TestRetry_NilConfig tests nil config uses defaults
func TestRetry_NilConfig(t *testing.T) {
	fn := func(ctx context.Context) error {
		return nil
	}

	err := Retry(context.Background(), nil, fn)
	assert.NoError(t, err)
}

// TestRetry_PermanentErrorNotInRetryableList tests permanent error not in retryable list
func TestRetry_PermanentErrorNotInRetryableList(t *testing.T) {
	callCount := 0
	errPermanent := errors.New("permanent error")
	errTemp := errors.New("temporary error")

	fn := func(ctx context.Context) error {
		callCount++
		return errTemp
	}

	config := &RetryConfig{
		MaxAttempts:     5,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		Multiplier:      2.0,
		MaxJitter:       0,
		RetryableErrors: []error{errPermanent}, // only permanent errors are retryable
	}

	err := Retry(context.Background(), config, fn)

	// errTemp is NOT in RetryableErrors, should NOT retry
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)
	assert.Contains(t, err.Error(), "non-retryable error")
}

// TestRetry_AllErrorsRetryableByDefault tests all errors are retryable by default
func TestRetry_AllErrorsRetryableByDefault(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return errors.New("any error")
	}

	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxJitter:    0,
		// No RetryableErrors specified, all errors should be retryable
	}

	err := Retry(context.Background(), config, fn)

	assert.Error(t, err)
	assert.Equal(t, 3, callCount)
}

// TestDefaultRetryConfig tests default configuration values
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	require.NotNil(t, config)
	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Equal(t, 500*time.Millisecond, config.MaxJitter)
}
