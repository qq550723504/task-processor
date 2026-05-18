// Package api 提供速率限制器实现
package api

import (
	"context"
	"math"
	"strings"
	"time"

	"task-processor/internal/core/logger"
	infraresilience "task-processor/internal/infra/resilience"

	"github.com/sirupsen/logrus"
)

// RateLimiter 速率限制器接口
type RateLimiter interface {
	Wait(ctx context.Context) error
	Allow() bool
}

// TokenBucketLimiter 令牌桶速率限制器
type TokenBucketLimiter struct {
	limiter *infraresilience.RateLimiter
}

// NewTokenBucketLimiter 创建令牌桶限制器。
//
// capacity 保持 float64 仅用于兼容 amazon/api 既有调用方；底层
// golang.org/x/time/rate 只接受整数 burst，因此这里按向上取整映射。
// 当前调用方传入的都是整数容量值，这个适配仅把历史构造签名显式化。
func NewTokenBucketLimiter(ratePerSecond, capacity float64) *TokenBucketLimiter {
	burst := int(math.Ceil(capacity))
	if burst < 1 {
		burst = 1
	}

	return &TokenBucketLimiter{
		limiter: infraresilience.NewRateLimiter(ratePerSecond, burst),
	}
}

// Wait 等待直到可以执行请求
func (r *TokenBucketLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

// Allow 检查是否允许执行请求
func (r *TokenBucketLimiter) Allow() bool {
	return r.limiter.Allow()
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
	case strings.Contains(path, "/feeds/"):
		return limits.Feeds
	case strings.Contains(path, "/reports/"):
		return limits.Reports
	case strings.Contains(path, "/listings/"):
		return limits.Listings
	case strings.Contains(path, "/pricing/"):
		return limits.Pricing
	case strings.Contains(path, "/inventory/"):
		return limits.Inventory
	case strings.Contains(path, "/orders/"):
		return limits.Orders
	case strings.Contains(path, "/catalog/"):
		return limits.Catalog
	case strings.Contains(path, "/uploads/"):
		return limits.Uploads
	default:
		return limits.Default
	}
}

// CircuitBreaker 断路器实现
type CircuitBreaker struct {
	breaker *infraresilience.CircuitBreaker
	logger  *logrus.Entry
}

// NewCircuitBreaker 创建断路器
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	logEntry := logger.GetGlobalLogger("CircuitBreaker")

	return &CircuitBreaker{
		breaker: infraresilience.NewCircuitBreaker(infraresilience.CircuitBreakerConfig{
			Name:             "amazon-api",
			MaxRequests:      normalizeThreshold(successThreshold),
			OpenTimeout:      timeout,
			ReadyToTripAfter: normalizeThreshold(failureThreshold),
			OnStateChange: func(from, to infraresilience.CircuitState) {
				switch to {
				case infraresilience.StateHalfOpen:
					logEntry.Info("断路器进入半开状态")
				case infraresilience.StateClosed:
					logEntry.Info("断路器关闭")
				case infraresilience.StateOpen:
					logEntry.WithFields(logrus.Fields{
						"from_state":        from,
						"failure_threshold": failureThreshold,
						"success_threshold": successThreshold,
						"open_timeout":      timeout.String(),
					}).Warn("断路器开启")
				}
			},
		}),
		logger: logEntry,
	}
}

// Execute 执行操作（带断路器保护）
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	return cb.breaker.Execute(ctx, func(context.Context) error {
		return operation()
	})
}

func normalizeThreshold(value int) uint32 {
	if value < 1 {
		return 1
	}
	return uint32(value)
}
