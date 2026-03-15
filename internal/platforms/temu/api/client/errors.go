// Package client 提供TEMU平台API客户端错误类型
package client

import "fmt"

// AuthExpiredError 认证过期错误（需要暂停任务等待Cookie更新）
type AuthExpiredError struct {
	Message string
	Cause   error
}

func (e *AuthExpiredError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AuthExpiredError) Unwrap() error {
	return e.Cause
}

// IsAuthExpired 标记接口，用于跨包检测认证过期错误
func (e *AuthExpiredError) IsAuthExpired() bool { return true }

// NewAuthExpiredError 创建认证过期错误
func NewAuthExpiredError(message string, cause error) *AuthExpiredError {
	return &AuthExpiredError{
		Message: message,
		Cause:   cause,
	}
}
