// Package errors 提供统一的错误处理机制
package errors

import (
	"fmt"
	"runtime"
	"time"
)

// ErrorCode 错误码类型
type ErrorCode string

const (
	// 系统级错误
	ErrCodeSystem        ErrorCode = "SYSTEM_ERROR"
	ErrCodeConfig        ErrorCode = "CONFIG_ERROR"
	ErrCodeAuth          ErrorCode = "AUTH_ERROR"
	ErrCodeNetwork       ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeout       ErrorCode = "TIMEOUT_ERROR"
	ErrCodeResourceLimit ErrorCode = "RESOURCE_LIMIT_ERROR"

	// 业务级错误
	ErrCodeTaskNotFound   ErrorCode = "TASK_NOT_FOUND"
	ErrCodeTaskDuplicate  ErrorCode = "TASK_DUPLICATE"
	ErrCodeTaskProcessing ErrorCode = "TASK_PROCESSING"
	ErrCodePlatformError  ErrorCode = "PLATFORM_ERROR"
	ErrCodeValidation     ErrorCode = "VALIDATION_ERROR"

	// 外部服务错误
	ErrCodeExternalAPI   ErrorCode = "EXTERNAL_API_ERROR"
	ErrCodeAmazonAPI     ErrorCode = "AMAZON_API_ERROR"
	ErrCodeManagementAPI ErrorCode = "MANAGEMENT_API_ERROR"
	ErrCodeOpenAIAPI     ErrorCode = "OPENAI_API_ERROR"
)

// AppError 应用错误结构
type AppError struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Cause     error     `json:"-"`
	Timestamp time.Time `json:"timestamp"`
	File      string    `json:"file,omitempty"`
	Line      int       `json:"line,omitempty"`
	Stack     string    `json:"stack,omitempty"`
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 支持errors.Unwrap
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is 支持errors.Is
func (e *AppError) Is(target error) bool {
	if appErr, ok := target.(*AppError); ok {
		return e.Code == appErr.Code
	}
	return false
}

// New 创建新的应用错误
func New(code ErrorCode, message string) *AppError {
	return newAppError(code, message, nil, "")
}

// Newf 创建格式化的应用错误
func Newf(code ErrorCode, format string, args ...interface{}) *AppError {
	return newAppError(code, fmt.Sprintf(format, args...), nil, "")
}

// Wrap 包装现有错误
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	return newAppError(code, message, err, "")
}

// Wrapf 包装现有错误并格式化消息
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *AppError {
	if err == nil {
		return nil
	}
	return newAppError(code, fmt.Sprintf(format, args...), err, "")
}

// WithDetails 添加详细信息
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithStack 添加堆栈信息
func (e *AppError) WithStack() *AppError {
	e.Stack = getStack()
	return e
}

// newAppError 创建应用错误的内部方法
func newAppError(code ErrorCode, message string, cause error, details string) *AppError {
	// 获取调用者信息
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	return &AppError{
		Code:      code,
		Message:   message,
		Details:   details,
		Cause:     cause,
		Timestamp: time.Now(),
		File:      file,
		Line:      line,
	}
}

// getStack 获取堆栈信息
func getStack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

// IsCode 检查错误是否为指定错误码
func IsCode(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetCode 获取错误码
func GetCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeSystem
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeNetwork, ErrCodeTimeout, ErrCodeExternalAPI:
			return true
		case ErrCodeResourceLimit:
			return true
		default:
			return false
		}
	}
	return false
}

// IsCritical 判断错误是否为关键错误
func IsCritical(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeSystem, ErrCodeConfig, ErrCodeAuth:
			return true
		default:
			return false
		}
	}
	return false
}
