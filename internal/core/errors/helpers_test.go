package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockLogger 模拟日志器
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Error(args ...interface{})                 { m.logs = append(m.logs, "ERROR") }
func (m *mockLogger) Errorf(format string, args ...interface{}) { m.logs = append(m.logs, "ERROR") }
func (m *mockLogger) Warn(args ...interface{})                  { m.logs = append(m.logs, "WARN") }
func (m *mockLogger) Warnf(format string, args ...interface{})  { m.logs = append(m.logs, "WARN") }
func (m *mockLogger) Info(args ...interface{})                  { m.logs = append(m.logs, "INFO") }
func (m *mockLogger) Infof(format string, args ...interface{})  { m.logs = append(m.logs, "INFO") }

func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		if attempts < 3 {
			return New(ErrCodeNetwork, "temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		return New(ErrCodeValidation, "validation error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultRetryConfig()
	config.InitialDelay = 100 * time.Millisecond

	// 在第一次重试前取消
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, config, func() error {
		return New(ErrCodeNetwork, "network error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !IsCode(err, ErrCodeTimeout) {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestSafeExecute_Success(t *testing.T) {
	logger := &mockLogger{}

	err := SafeExecute(func() error {
		return nil
	}, logger)

	if err != nil {
		t.Errorf("Expected nil, got error: %v", err)
	}
}

func TestSafeExecute_Error(t *testing.T) {
	logger := &mockLogger{}

	expectedErr := errors.New("test error")
	err := SafeExecute(func() error {
		return expectedErr
	}, logger)

	if err != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

func TestSafeExecute_Panic(t *testing.T) {
	logger := &mockLogger{}

	err := SafeExecute(func() error {
		panic("test panic")
	}, logger)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if len(logger.logs) == 0 {
		t.Error("Expected log entry")
	}
}

func TestCombine(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expected int // expected number of errors
	}{
		{
			name:     "no errors",
			errors:   []error{},
			expected: 0,
		},
		{
			name:     "single error",
			errors:   []error{errors.New("error1")},
			expected: 1,
		},
		{
			name:     "multiple errors",
			errors:   []error{errors.New("error1"), errors.New("error2")},
			expected: 2,
		},
		{
			name:     "mixed with nil",
			errors:   []error{errors.New("error1"), nil, errors.New("error2")},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Combine(tt.errors...)

			if tt.expected == 0 {
				if err != nil {
					t.Errorf("Expected nil, got %v", err)
				}
				return
			}

			if err == nil {
				t.Error("Expected error, got nil")
				return
			}

			if tt.expected > 1 {
				multiErr, ok := err.(*MultiError)
				if !ok {
					t.Errorf("Expected MultiError, got %T", err)
					return
				}

				if len(multiErr.Errors) != tt.expected {
					t.Errorf("Expected %d errors, got %d", tt.expected, len(multiErr.Errors))
				}
			}
		})
	}
}

func TestIgnoreError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		ignoreCodes  []ErrorCode
		shouldIgnore bool
	}{
		{
			name:         "nil error",
			err:          nil,
			ignoreCodes:  []ErrorCode{ErrCodeValidation},
			shouldIgnore: true,
		},
		{
			name:         "ignore validation error",
			err:          New(ErrCodeValidation, "validation failed"),
			ignoreCodes:  []ErrorCode{ErrCodeValidation},
			shouldIgnore: true,
		},
		{
			name:         "don't ignore network error",
			err:          New(ErrCodeNetwork, "network failed"),
			ignoreCodes:  []ErrorCode{ErrCodeValidation},
			shouldIgnore: false,
		},
		{
			name:         "non-AppError",
			err:          errors.New("standard error"),
			ignoreCodes:  []ErrorCode{ErrCodeValidation},
			shouldIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IgnoreError(tt.err, tt.ignoreCodes...)

			if tt.shouldIgnore && result != nil {
				t.Errorf("Expected nil, got %v", result)
			}

			if !tt.shouldIgnore && result == nil && tt.err != nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestDefaultErrorHandler(t *testing.T) {
	logger := &mockLogger{}
	handler := NewDefaultErrorHandler(logger)

	tests := []struct {
		name            string
		err             error
		shouldRetry     bool
		shouldTerminate bool
	}{
		{
			name:            "nil error",
			err:             nil,
			shouldRetry:     false,
			shouldTerminate: false,
		},
		{
			name:            "retryable error",
			err:             New(ErrCodeNetwork, "network error"),
			shouldRetry:     true,
			shouldTerminate: false,
		},
		{
			name:            "critical error",
			err:             New(ErrCodeSystem, "system error"),
			shouldRetry:     false,
			shouldTerminate: true,
		},
		{
			name:            "validation error",
			err:             New(ErrCodeValidation, "validation error"),
			shouldRetry:     false,
			shouldTerminate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Handle(tt.err)

			if tt.err == nil && result != nil {
				t.Errorf("Expected nil, got %v", result)
			}

			if handler.ShouldRetry(tt.err) != tt.shouldRetry {
				t.Errorf("ShouldRetry: expected %v, got %v", tt.shouldRetry, handler.ShouldRetry(tt.err))
			}

			if handler.ShouldTerminate(tt.err) != tt.shouldTerminate {
				t.Errorf("ShouldTerminate: expected %v, got %v", tt.shouldTerminate, handler.ShouldTerminate(tt.err))
			}
		})
	}
}
