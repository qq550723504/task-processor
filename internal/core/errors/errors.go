// Package errors 提供统一的错误处理机制
package errors

import (
	"errors"
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
	return newAppError(code, message, nil)
}

// Newf 创建格式化的应用错误
func Newf(code ErrorCode, format string, args ...any) *AppError {
	return newAppError(code, fmt.Sprintf(format, args...), nil)
}

// Wrap 包装现有错误
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	return newAppError(code, message, err)
}

// Wrapf 包装现有错误并格式化消息
func Wrapf(err error, code ErrorCode, format string, args ...any) *AppError {
	if err == nil {
		return nil
	}
	return newAppError(code, fmt.Sprintf(format, args...), err)
}

// WithDetails 添加详细信息
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithFileLine 添加文件和行号
func (e *AppError) WithFileLine(file string, line int) *AppError {
	e.File = file
	e.Line = line
	return e
}

// WithStack 添加堆栈信息
func (e *AppError) WithStack() *AppError {
	e.Stack = getStack()
	return e
}

// newAppError 创建应用错误的内部方法
func newAppError(code ErrorCode, message string, cause error) *AppError {
	// 获取调用者信息
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	return &AppError{
		Code:      code,
		Message:   message,
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
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// GetCode 获取错误码
func GetCode(err error) ErrorCode {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ErrCodeSystem
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
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
	var appErr *AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case ErrCodeSystem, ErrCodeConfig, ErrCodeAuth:
			return true
		default:
			return false
		}
	}
	return false
}

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	Handle(err error) error
	ShouldRetry(err error) bool
	ShouldTerminate(err error) bool
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	logger Logger
}

// Logger 日志接口
type Logger interface {
	Error(args ...any)
	Errorf(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
}

// NewDefaultErrorHandler 创建默认错误处理器
func NewDefaultErrorHandler(logger Logger) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// Handle 处理错误
func (h *DefaultErrorHandler) Handle(err error) error {
	if err == nil {
		return nil
	}

	// 记录错误日志
	var appErr *AppError
	if errors.As(err, &appErr) {
		if IsCritical(err) {
			h.logger.Errorf("[CRITICAL] %s", appErr.Error())
		} else if IsRetryable(err) {
			h.logger.Warnf("[RETRYABLE] %s", appErr.Error())
		} else {
			h.logger.Errorf("[ERROR] %s", appErr.Error())
		}
		return appErr
	}

	// 非AppError，包装后返回
	h.logger.Errorf("[ERROR] %v", err)
	return Wrap(err, ErrCodeSystem, "未知错误")
}

// ShouldRetry 判断是否应该重试
func (h *DefaultErrorHandler) ShouldRetry(err error) bool {
	return IsRetryable(err)
}

// ShouldTerminate 判断是否应该终止
func (h *DefaultErrorHandler) ShouldTerminate(err error) bool {
	return IsCritical(err)
}

// RecoverFromPanic 从panic中恢复
func RecoverFromPanic(logger Logger) {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = New(ErrCodeSystem, x)
		case error:
			err = Wrap(x, ErrCodeSystem, "panic recovered")
		default:
			err = Newf(ErrCodeSystem, "panic recovered: %v", x)
		}

		if logger != nil {
			logger.Errorf("Panic recovered: %v", err)
		}
	}
}

// Must panics if error is not nil (ONLY use in initialization phase: main or init functions)
// This is a convenience function for initialization code where errors are unrecoverable.
// DO NOT use in business logic - return errors instead.
func Must(err error) {
	if err != nil {
		panic(fmt.Errorf("initialization failed: %w", err))
	}
}

// MustValue panics if error is not nil, otherwise returns value (ONLY use in initialization phase)
// This is a convenience function for initialization code where errors are unrecoverable.
// DO NOT use in business logic - return errors instead.
func MustValue[T any](value T, err error) T {
	if err != nil {
		panic(fmt.Errorf("initialization failed: %w", err))
	}
	return value
}
