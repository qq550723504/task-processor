// Package pipeline 提供管道相关错误定义
package pipeline

import "fmt"

// ProcessError 管道处理错误，统一用于 handler 和 pipeline 层。
type ProcessError struct {
	Source  string // handler 名称或 pipeline 名称
	Message string
	Cause   error
}

// NewProcessError 创建处理错误
func NewProcessError(source, message string, cause error) *ProcessError {
	return &ProcessError{Source: source, Message: message, Cause: cause}
}

// Error 实现 error 接口
func (e *ProcessError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Source, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Source, e.Message)
}

// Unwrap 支持错误链
func (e *ProcessError) Unwrap() error {
	return e.Cause
}
