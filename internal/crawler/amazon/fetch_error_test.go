package amazon

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyFetchError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  string
		retryable bool
	}{
		{
			name:      "invalid request",
			err:       errors.New("url or asin is required"),
			wantType:  FetchErrorTypeInvalidRequest,
			retryable: false,
		},
		{
			name:      "captcha",
			err:       errors.New("captcha challenge detected"),
			wantType:  FetchErrorTypeCaptcha,
			retryable: false,
		},
		{
			name:      "timeout",
			err:       errors.New("navigation timeout exceeded"),
			wantType:  FetchErrorTypeTimeout,
			retryable: true,
		},
		{
			name:      "dedupe wait",
			err:       errors.New("crawl already in progress and shared result timed out"),
			wantType:  FetchErrorTypeCrawlInProgress,
			retryable: true,
		},
		{
			name:      "processor unavailable",
			err:       errors.New("初始化浏览器池失败: chromium launch failed"),
			wantType:  FetchErrorTypeProcessorUnavailable,
			retryable: true,
		},
		{
			name:      "typed system busy",
			err:       newSystemBusyError("crawler concurrency acquire timeout: context deadline exceeded", context.DeadlineExceeded),
			wantType:  FetchErrorTypeSystemBusy,
			retryable: true,
		},
		{
			name:      "typed invalid request",
			err:       newInvalidRequestError("url or asin is required"),
			wantType:  FetchErrorTypeInvalidRequest,
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyFetchError(tt.err)
			if got.ErrorType() != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, got.ErrorType())
			}
			if got.RetryableError() != tt.retryable {
				t.Fatalf("expected retryable=%v, got %v", tt.retryable, got.RetryableError())
			}
		})
	}
}
