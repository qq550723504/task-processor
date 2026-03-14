// Package resilience 提供重试机制工具
package resilience

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts     int           // 最大重试次数
	InitialDelay    time.Duration // 初始延迟
	MaxDelay        time.Duration // 最大延迟
	Multiplier      float64       // 延迟倍数
	MaxJitter       time.Duration // 最大抖动时间
	RetryableErrors []error       // 可重试的错误类型
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxJitter:    500 * time.Millisecond,
	}
}

// RetryFunc 可重试的函数类型
type RetryFunc func(ctx context.Context) error

// Retry 执行带重试的函数
func Retry(ctx context.Context, config *RetryConfig, fn RetryFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// 执行函数
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// 最后一次尝试，不再重试
		if attempt == config.MaxAttempts {
			break
		}

		// 检查是否可重试
		if !isRetryable(err, config.RetryableErrors) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// 计算延迟时间（指数退避 + 抖动）
		actualDelay := calculateDelay(delay, config.MaxDelay, config.MaxJitter)

		// 等待后重试
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(actualDelay):
			// 增加延迟
			delay = time.Duration(float64(delay) * config.Multiplier)
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// isRetryable 检查错误是否可重试
func isRetryable(err error, retryableErrors []error) bool {
	if len(retryableErrors) == 0 {
		// 如果没有指定可重试错误，默认所有错误都可重试
		return true
	}

	for _, retryableErr := range retryableErrors {
		if err == retryableErr {
			return true
		}
	}

	return false
}

// calculateDelay 计算实际延迟时间（带抖动）
func calculateDelay(delay, maxDelay, maxJitter time.Duration) time.Duration {
	// 限制最大延迟
	if delay > maxDelay {
		delay = maxDelay
	}

	// 添加随机抖动
	if maxJitter > 0 {
		jitter := time.Duration(float64(maxJitter) * (0.5 + 0.5*randomFloat()))
		delay += jitter
	}

	return delay
}

// randomFloat 生成 0-1 之间的随机浮点数
func randomFloat() float64 {
	// 简单实现，生产环境应使用 crypto/rand
	return float64(time.Now().UnixNano()%1000) / 1000.0
}
