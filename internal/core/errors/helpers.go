// Package errors 提供统一的错误处理辅助函数
package errors

import (
	"context"
	"errors"
	"fmt"
	"task-processor/internal/core/config"
	"time"
)

// ErrorsRetryConfig 错误处理的重试配置（扩展通用配置）
type ErrorsRetryConfig struct {
	*config.RetryConfig
	RetryableCheck func(error) bool
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() ErrorsRetryConfig {
	return ErrorsRetryConfig{
		RetryConfig: &config.RetryConfig{
			MaxRetries:    3,
			InitialDelay:  time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
		RetryableCheck: IsRetryable,
	}
}

// Retry 重试执行函数
func Retry(ctx context.Context, cfg ErrorsRetryConfig, fn func() error) error {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// 检查context是否已取消
		select {
		case <-ctx.Done():
			return Wrap(ctx.Err(), ErrCodeTimeout, "操作被取消")
		default:
		}

		// 执行函数
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否可重试
		if !cfg.RetryableCheck(err) {
			return err
		}

		// 最后一次尝试失败
		if attempt == cfg.MaxRetries {
			break
		}

		// 等待后重试
		select {
		case <-ctx.Done():
			return Wrap(ctx.Err(), ErrCodeTimeout, "重试被取消")
		case <-time.After(delay):
		}

		// 计算下次延迟（指数退避）
		delay = time.Duration(float64(delay) * cfg.BackoffFactor)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return Wrapf(lastErr, ErrCodeSystem, "重试%d次后仍然失败", cfg.MaxRetries)
}

// SafeExecute 安全执行函数，捕获panic
func SafeExecute(fn func() error, logger Logger) (err error) {
	defer func() {
		if r := recover(); r != nil {
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
	}()

	return fn()
}

// SafeGo 安全启动goroutine
func SafeGo(fn func(), logger Logger) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch x := r.(type) {
				case string:
					err = New(ErrCodeSystem, x)
				case error:
					err = Wrap(x, ErrCodeSystem, "goroutine panic")
				default:
					err = Newf(ErrCodeSystem, "goroutine panic: %v", x)
				}

				if logger != nil {
					logger.Errorf("Goroutine panic: %v", err)
				}
			}
		}()

		fn()
	}()
}

// Combine 合并多个错误
func Combine(errs ...error) error {
	var validErrs []error
	for _, err := range errs {
		if err != nil {
			validErrs = append(validErrs, err)
		}
	}

	if len(validErrs) == 0 {
		return nil
	}

	if len(validErrs) == 1 {
		return validErrs[0]
	}

	return &MultiError{Errors: validErrs}
}

// MultiError 多个错误的集合
type MultiError struct {
	Errors []error
}

// Error 实现error接口
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}

	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	msg := fmt.Sprintf("multiple errors (%d):", len(e.Errors))
	for i, err := range e.Errors {
		msg += fmt.Sprintf("\n  %d. %v", i+1, err)
	}
	return msg
}

// Unwrap 返回第一个错误
func (e *MultiError) Unwrap() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[0]
}

// IgnoreError 忽略特定错误
func IgnoreError(err error, ignoreCodes ...ErrorCode) error {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if !errors.As(err, &appErr) {
		return err
	}

	for _, code := range ignoreCodes {
		if appErr.Code == code {
			return nil
		}
	}

	return err
}

// WrapWithContext 包装错误并添加上下文信息
func WrapWithContext(err error, code ErrorCode, context map[string]any) *AppError {
	if err == nil {
		return nil
	}

	details := ""
	if len(context) > 0 {
		details = fmt.Sprintf("Context: %+v", context)
	}

	appErr := Wrap(err, code, err.Error())
	if details != "" {
		appErr.Details = details
	}

	return appErr
}
