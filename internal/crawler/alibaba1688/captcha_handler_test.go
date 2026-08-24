package alibaba1688

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCaptchaManualFallbackCanBeDisabled(t *testing.T) {
	handler := NewCaptchaHandler()

	result := handler.fallbackCaptchaResult(context.Background(), nil, CaptchaTypeSlider, "滑动验证码", false)
	if result.Status != CaptchaStatusFailed {
		t.Fatalf("public captcha fallback status = %d, want failed", result.Status)
	}
	if result.UsedMethod != "automatic_only" {
		t.Fatalf("public captcha fallback method = %q, want automatic_only", result.UsedMethod)
	}
}

func TestHandlePageCaptchaReturnsCancellationPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	err := NewCaptchaHandler().HandlePageCaptcha(ctx, nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("HandlePageCaptcha() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
		t.Fatalf("HandlePageCaptcha() took %v after cancellation", elapsed)
	}
}

func TestWaitForManualCaptchaReturnsCancellationPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	result := NewCaptchaHandler().waitForManualCaptchaWithResult(ctx, nil, CaptchaTypeText, "文字验证码")

	if !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("manual captcha error = %v, want context.Canceled", result.Error)
	}
	if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
		t.Fatalf("manual captcha wait took %v after cancellation", elapsed)
	}
}
