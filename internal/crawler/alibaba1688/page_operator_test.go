package alibaba1688

import (
	"errors"
	"strings"
	"testing"
)

func TestCaptchaStageErrorPropagates(t *testing.T) {
	err := captchaStageError("验证码处理", errors.New("等待用户手动操作超时"))
	if err == nil {
		t.Fatal("captcha stage error should be propagated")
	}
	if !strings.Contains(err.Error(), "验证码处理失败") {
		t.Fatalf("error = %q, want captcha context", err.Error())
	}
	if !strings.Contains(err.Error(), "等待用户手动操作超时") {
		t.Fatalf("error = %q, want original cause", err.Error())
	}
}

func TestCaptchaStageErrorAllowsNil(t *testing.T) {
	if err := captchaStageError("验证码处理", nil); err != nil {
		t.Fatalf("nil captcha error = %v, want nil", err)
	}
}
