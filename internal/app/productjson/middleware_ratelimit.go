// Package productjson 提供 API 处理器和中间件
package productjson

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(key string) bool
}

// tokenBucketLimiter 令牌桶限流器
type tokenBucketLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(r rate.Limit, burst int) RateLimiter {
	return &tokenBucketLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    burst,
	}
}

// Allow 检查是否允许请求
func (l *tokenBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	limiter, exists := l.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(l.rate, l.burst)
		l.limiters[key] = limiter
	}
	l.mu.Unlock()

	return limiter.Allow()
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 使用 IP 地址作为限流 key
		key := c.ClientIP()

		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByUserMiddleware 按用户限流中间件
func RateLimitByUserMiddleware(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户 ID（需要先经过认证中间件）
		userID, exists := c.Get("user_id")
		if !exists {
			// 如果没有用户 ID，使用 IP 地址
			userID = c.ClientIP()
		}

		key := userID.(string)

		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// slidingWindowLimiter 滑动窗口限流器
type slidingWindowLimiter struct {
	windows map[string]*window
	mu      sync.RWMutex
	limit   int
	window  time.Duration
}

// window 时间窗口
type window struct {
	requests []time.Time
	mu       sync.Mutex
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(limit int, windowDuration time.Duration) RateLimiter {
	limiter := &slidingWindowLimiter{
		windows: make(map[string]*window),
		limit:   limit,
		window:  windowDuration,
	}

	// 启动清理协程
	go limiter.cleanup()

	return limiter
}

// Allow 检查是否允许请求
func (l *slidingWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	w, exists := l.windows[key]
	if !exists {
		w = &window{
			requests: make([]time.Time, 0),
		}
		l.windows[key] = w
	}
	l.mu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// 移除过期的请求
	validRequests := make([]time.Time, 0)
	for _, t := range w.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	w.requests = validRequests

	// 检查是否超过限制
	if len(w.requests) >= l.limit {
		return false
	}

	// 添加当前请求
	w.requests = append(w.requests, now)
	return true
}

// cleanup 清理过期的窗口
func (l *slidingWindowLimiter) cleanup() {
	ticker := time.NewTicker(l.window)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.window * 2)

		for key, w := range l.windows {
			w.mu.Lock()
			if len(w.requests) == 0 || w.requests[len(w.requests)-1].Before(cutoff) {
				delete(l.windows, key)
			}
			w.mu.Unlock()
		}
		l.mu.Unlock()
	}
}
