// Package pipeline 提供管道相关错误定义
package pipeline

import "fmt"

// HandlerError 处理器错误
type HandlerError struct {
	HandlerName string
	Message     string
	Cause       error
}

// NewHandlerError 创建处理器错误
func NewHandlerError(handlerName, message string) *HandlerError {
	return &HandlerError{
		HandlerName: handlerName,
		Message:     message,
	}
}

// NewHandlerErrorWithCause 创建带原因的处理器错误
func NewHandlerErrorWithCause(handlerName, message string, cause error) *HandlerError {
	return &HandlerError{
		HandlerName: handlerName,
		Message:     message,
		Cause:       cause,
	}
}

// Error 实现error接口
func (e *HandlerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.HandlerName, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.HandlerName, e.Message)
}

// Unwrap 支持错误链
func (e *HandlerError) Unwrap() error {
	return e.Cause
}

// PipelineError 管道错误
type PipelineError struct {
	PipelineName string
	Message      string
	Cause        error
}

// NewPipelineError 创建管道错误
func NewPipelineError(pipelineName, message string) *PipelineError {
	return &PipelineError{
		PipelineName: pipelineName,
		Message:      message,
	}
}

// NewPipelineErrorWithCause 创建带原因的管道错误
func NewPipelineErrorWithCause(pipelineName, message string, cause error) *PipelineError {
	return &PipelineError{
		PipelineName: pipelineName,
		Message:      message,
		Cause:        cause,
	}
}

// Error 实现error接口
func (e *PipelineError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[管道:%s] %s: %v", e.PipelineName, e.Message, e.Cause)
	}
	return fmt.Sprintf("[管道:%s] %s", e.PipelineName, e.Message)
}

// Unwrap 支持错误链
func (e *PipelineError) Unwrap() error {
	return e.Cause
}
