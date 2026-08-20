package alibaba1688

import "testing"

func TestCaptchaManualFallbackCanBeDisabled(t *testing.T) {
	handler := NewCaptchaHandler()

	result := handler.fallbackCaptchaResult(nil, CaptchaTypeSlider, "滑动验证码", false)
	if result.Status != CaptchaStatusFailed {
		t.Fatalf("public captcha fallback status = %d, want failed", result.Status)
	}
	if result.UsedMethod != "automatic_only" {
		t.Fatalf("public captcha fallback method = %q, want automatic_only", result.UsedMethod)
	}
}
