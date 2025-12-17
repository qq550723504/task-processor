package modules

import (
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	// 测试可重试错误
	retryableErr := NewRetryableError("test retryable error", nil)
	if !IsRetryableError(retryableErr) {
		t.Errorf("Expected retryable error to be retryable, but it wasn't")
	}

	// 测试不可重试错误
	nonRetryableErr := NewNonRetryableError("test non-retryable error", nil)
	if IsRetryableError(nonRetryableErr) {
		t.Errorf("Expected non-retryable error to not be retryable, but it was")
	}
}
