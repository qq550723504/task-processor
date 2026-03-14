// Package apperr 提供应用错误类型
package apperr

import (
	"fmt"
	"runtime"
)

// AppError 应用错误
type AppError struct {
	Code    string
	Message string
	Cause   error
	File    string
	Line    int
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v (at %s:%d)", e.Code, e.Message, e.Cause, e.File, e.Line)
	}
	return fmt.Sprintf("[%s] %s (at %s:%d)", e.Code, e.Message, e.File, e.Line)
}

func (e *AppError) Unwrap() error { return e.Cause }

// New 创建应用错误
func New(code, message string, cause error) *AppError {
	_, file, line, _ := runtime.Caller(1)
	return &AppError{Code: code, Message: message, Cause: cause, File: file, Line: line}
}

// Wrap 包装错误
func Wrap(err error, code, message string) *AppError {
	if err == nil {
		return nil
	}
	return New(code, message, err)
}

// 常用错误代码
const (
	ErrCodeConfig     = "CONFIG_ERROR"
	ErrCodeAuth       = "AUTH_ERROR"
	ErrCodeServer     = "SERVER_ERROR"
	ErrCodeCrawler    = "CRAWLER_ERROR"
	ErrCodeValidation = "VALIDATION_ERROR"
)
