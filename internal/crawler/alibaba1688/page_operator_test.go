package alibaba1688

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestWaitForContextReturnsCancellationPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := waitForContext(ctx, 2*time.Second)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), 200*time.Millisecond)
}
