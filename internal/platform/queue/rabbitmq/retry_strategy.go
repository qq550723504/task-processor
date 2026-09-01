// Package rabbitmq 提供重试策略
package rabbitmq

import (
	"math"
	"time"
)

// RetryStrategy 重试策略接口
type RetryStrategy interface {
	// NextDelay 计算下一次重试的延迟时间
	NextDelay(attempt int) time.Duration
	// ShouldRetry 判断是否应该继续重试
	ShouldRetry(attempt int, err error) bool
}

// FixedDelayStrategy 固定延迟重试策略
type FixedDelayStrategy struct {
	Delay      time.Duration
	MaxRetries int
}

// NextDelay 返回固定的延迟时间
func (s *FixedDelayStrategy) NextDelay(attempt int) time.Duration {
	return s.Delay
}

// ShouldRetry 判断是否应该继续重试
func (s *FixedDelayStrategy) ShouldRetry(attempt int, err error) bool {
	return attempt < s.MaxRetries
}

// ExponentialBackoffStrategy 指数退避重试策略
type ExponentialBackoffStrategy struct {
	InitialDelay time.Duration // 初始延迟
	MaxDelay     time.Duration // 最大延迟
	Multiplier   float64       // 倍数（通常为2.0）
	MaxRetries   int           // 最大重试次数
}

// NextDelay 计算指数退避的延迟时间
func (s *ExponentialBackoffStrategy) NextDelay(attempt int) time.Duration {
	// 计算延迟: InitialDelay * (Multiplier ^ attempt)
	delay := time.Duration(float64(s.InitialDelay) * math.Pow(s.Multiplier, float64(attempt)))

	// 限制最大延迟
	if delay > s.MaxDelay {
		delay = s.MaxDelay
	}

	return delay
}

// ShouldRetry 判断是否应该继续重试
func (s *ExponentialBackoffStrategy) ShouldRetry(attempt int, err error) bool {
	return attempt < s.MaxRetries
}

// NewDefaultRetryStrategy 创建默认的重试策略（指数退避）
func NewDefaultRetryStrategy(maxRetries int) RetryStrategy {
	return &ExponentialBackoffStrategy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   maxRetries,
	}
}
