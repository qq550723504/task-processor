// Package api 提供速率限制器实现
package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RateLimiter 速率限制器接口
type RateLimiter interface {
	Wait(ctx context.Context) error
	Allow() bool
}

// TokenBucketLimiter 令牌桶速率限制器
type TokenBucketLimiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64
	lastTime time.Time
	logger   *logrus.Entry
}

// NewTokenBucketLimiter 创建令牌桶限制器
func NewTokenBucketLimiter(rate, capacity float64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		tokens:   capacity,
		capacity: capacity,
		rate:     rate,
		lastTime: time.Now(),
		logger:   logrus.WithField("component", "RateLimiter"),
	}
}

// Wait 等待直到可以执行请求
func (r *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		if r.Allow() {
			return nil
		}

		// 计算需要等待的时间
		waitTime := time.Duration(1.0/r.rate*1000) * time.Millisecond

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			continue
		}
	}
}

// Allow 检查是否允许执行请求
func (r *TokenBucketLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()

	// 添加令牌
	r.tokens += elapsed * r.rate
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}

	r.lastTime = now

	// 检查是否有足够的令牌
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

// APIRateLimits Amazon SP-API速率限制配置
type APIRateLimits struct {
	// Selling Partner API的标准限制
	Default   RateLimiter // 默认限制: 10 requests/second
	Feeds     RateLimiter // Feeds API: 0.0167 requests/second (1 request/minute)
	Reports   RateLimiter // Reports API: 0.0167 requests/second
	Listings  RateLimiter // Listings API: 5 requests/second
	Pricing   RateLimiter // Pricing API: 10 requests/second
	Inventory RateLimiter // Inventory API: 30 requests/second
	Orders    RateLimiter // Orders API: 0.5 requests/second
	Catalog   RateLimiter // Catalog API: 2 requests/second
	Uploads   RateLimiter // Uploads API: 10 requests/second
}

// NewAPIRateLimits 创建API速率限制器集合
func NewAPIRateLimits() *APIRateLimits {
	return &APIRateLimits{
		Default:   NewTokenBucketLimiter(10.0, 20.0),  // 10 req/s, burst 20
		Feeds:     NewTokenBucketLimiter(0.0167, 1.0), // 1 req/min
		Reports:   NewTokenBucketLimiter(0.0167, 1.0), // 1 req/min
		Listings:  NewTokenBucketLimiter(5.0, 10.0),   // 5 req/s, burst 10
		Pricing:   NewTokenBucketLimiter(10.0, 20.0),  // 10 req/s, burst 20
		Inventory: NewTokenBucketLimiter(30.0, 60.0),  // 30 req/s, burst 60
		Orders:    NewTokenBucketLimiter(0.5, 2.0),    // 0.5 req/s, burst 2
		Catalog:   NewTokenBucketLimiter(2.0, 5.0),    // 2 req/s, burst 5
		Uploads:   NewTokenBucketLimiter(10.0, 20.0),  // 10 req/s, burst 20
	}
}

// GetLimiterForPath 根据API路径获取对应的速率限制器
func (limits *APIRateLimits) GetLimiterForPath(path string) RateLimiter {
	switch {
	case containsSubstring(path, "/feeds/"):
		return limits.Feeds
	case containsSubstring(path, "/reports/"):
		return limits.Reports
	case containsSubstring(path, "/listings/"):
		return limits.Listings
	case containsSubstring(path, "/pricing/"):
		return limits.Pricing
	case containsSubstring(path, "/inventory/"):
		return limits.Inventory
	case containsSubstring(path, "/orders/"):
		return limits.Orders
	case containsSubstring(path, "/catalog/"):
		return limits.Catalog
	case containsSubstring(path, "/uploads/"):
		return limits.Uploads
	default:
		return limits.Default
	}
}

// CircuitBreaker 断路器实现
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailureTime  time.Time
	logger           *logrus.Entry
}

// CircuitState 断路器状态
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// NewCircuitBreaker 创建断路器
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		logger:           logrus.WithField("component", "CircuitBreaker"),
	}
}

// Execute 执行操作（带断路器保护）
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	if !cb.allowRequest() {
		return fmt.Errorf("断路器开启，请求被拒绝")
	}

	err := operation()
	cb.recordResult(err == nil)

	return err
}

// allowRequest 检查是否允许请求
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以进入半开状态
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.logger.Info("断路器进入半开状态")
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult 记录操作结果
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.failureCount = 0
		cb.successCount++

		if cb.state == StateHalfOpen && cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.logger.Info("断路器关闭")
		}
	} else {
		cb.successCount = 0
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.failureCount >= cb.failureThreshold {
			cb.state = StateOpen
			cb.logger.WithField("failure_count", cb.failureCount).Warn("断路器开启")
		}
	}
}
