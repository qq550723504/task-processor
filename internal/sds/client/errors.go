package client

import "fmt"

// Error 表示 SDS 客户端统一错误。
type Error struct {
	Op         string
	StatusCode int
	Message    string
	Err        error
}

func (e *Error) Error() string {
	switch {
	case e.Err != nil && e.StatusCode > 0:
		return fmt.Sprintf("sds %s failed with status %d: %s: %v", e.Op, e.StatusCode, e.Message, e.Err)
	case e.Err != nil:
		return fmt.Sprintf("sds %s failed: %s: %v", e.Op, e.Message, e.Err)
	case e.StatusCode > 0:
		return fmt.Sprintf("sds %s failed with status %d: %s", e.Op, e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("sds %s failed: %s", e.Op, e.Message)
	}
}

func (e *Error) Unwrap() error {
	return e.Err
}

// AuthRequiredError 表示 SDS 登录态失效，需要刷新。
type AuthRequiredError struct {
	Op         string
	StatusCode int
	Message    string
}

func (e *AuthRequiredError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("sds %s auth required with status %d: %s", e.Op, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("sds %s auth required: %s", e.Op, e.Message)
}

// CaptchaRequiredError 表示 SDS 登录还停留在验证码/二段登录阶段。
type CaptchaRequiredError struct {
	Op          string
	Message     string
	RequestID   string
	VerifyCode  string
	VerifyState bool
}

func (e *CaptchaRequiredError) Error() string {
	base := fmt.Sprintf("sds %s requires captcha verification", e.Op)
	if e.Message != "" {
		base = fmt.Sprintf("%s: %s", base, e.Message)
	}
	if e.VerifyCode != "" {
		base = fmt.Sprintf("%s (verifyCode=%s, verified=%t)", base, e.VerifyCode, e.VerifyState)
	}
	if e.RequestID != "" {
		base = fmt.Sprintf("%s [requestId=%s]", base, e.RequestID)
	}
	return base
}
