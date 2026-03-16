// Package api 提供重试机制和速率限制处理
package api

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"time"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// AmazonRetryConfig Amazon特定的重试配置（扩展通用配置）
type AmazonRetryConfig struct {
	*config.RetryConfig
	RetryableErrors []string `json:"retryableErrors"`
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *AmazonRetryConfig {
	return &AmazonRetryConfig{
		RetryConfig: &config.RetryConfig{
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
		RetryableErrors: []string{
			"TooManyRequests",
			"InternalFailure",
			"ServiceUnavailable",
			"Throttled",
		},
	}
}

// RetryableRequest 可重试的请求函数类型
type RetryableRequest func(ctx context.Context) (*http.Response, error)

// ExecuteWithRetry 执行带重试的请求
func (c *Client) ExecuteWithRetry(ctx context.Context, operation string, request RetryableRequest) (*http.Response, error) {
	retryConfig := DefaultRetryConfig()

	var lastErr error
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算退避延迟
			delay := c.calculateBackoffDelay(retryConfig.RetryConfig, attempt)

			c.logger.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt,
				"delay":     delay,
			}).Warn("重试API请求")

			// 等待退避时间
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// 执行请求
		resp, err := request(ctx)
		if err != nil {
			lastErr = err

			// 检查是否应该重试
			if !c.shouldRetry(err, retryConfig) {
				return nil, fmt.Errorf("请求失败（不可重试）: %w", err)
			}

			c.logger.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt,
				"error":     err.Error(),
			}).Warn("请求失败，准备重试")

			continue
		}

		// 检查HTTP状态码
		if c.shouldRetryStatusCode(resp.StatusCode) {
			lastErr = fmt.Errorf("HTTP错误: %d", resp.StatusCode)

			// 处理速率限制
			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfter := c.parseRetryAfter(resp)
				if retryAfter > 0 {
					c.logger.WithFields(logrus.Fields{
						"operation":   operation,
						"retry_after": retryAfter,
					}).Warn("遇到速率限制，等待重试")

					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(retryAfter):
					}
				}
			}

			resp.Body.Close()
			continue
		}

		// 请求成功
		if attempt > 0 {
			c.logger.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt,
			}).Info("重试成功")
		}

		return resp, nil
	}

	return nil, fmt.Errorf("请求失败，已达到最大重试次数 (%d): %w", retryConfig.MaxRetries, lastErr)
}

// calculateBackoffDelay 计算指数退避延迟
func (c *Client) calculateBackoffDelay(retryConfig *config.RetryConfig, attempt int) time.Duration {
	delay := float64(retryConfig.InitialDelay) * math.Pow(retryConfig.BackoffFactor, float64(attempt-1))

	// 添加抖动（±25%）
	jitter := delay * 0.25 * (2*rand.Float64() - 1)
	delay += jitter

	// 限制最大延迟
	if delay > float64(retryConfig.MaxDelay) {
		delay = float64(retryConfig.MaxDelay)
	}

	return time.Duration(delay)
}

// shouldRetry 判断是否应该重试
func (c *Client) shouldRetry(err error, retryConfig *AmazonRetryConfig) bool {
	if err == nil {
		return false
	}

	// 检查是否是可重试的错误
	errStr := err.Error()
	for _, retryableErr := range retryConfig.RetryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// shouldRetryStatusCode 判断HTTP状态码是否应该重试
func (c *Client) shouldRetryStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// parseRetryAfter 解析Retry-After头部
func (c *Client) parseRetryAfter(resp *http.Response) time.Duration {
	retryAfterStr := resp.Header.Get("Retry-After")
	if retryAfterStr == "" {
		return 0
	}

	// 尝试解析为秒数
	if seconds, err := time.ParseDuration(retryAfterStr + "s"); err == nil {
		return seconds
	}

	// 尝试解析为时间戳（RFC1123格式）
	if timestamp, err := time.Parse(time.RFC1123, retryAfterStr); err == nil {
		return time.Until(timestamp)
	}

	return 0
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

// containsSubstring 检查字符串中间是否包含子字符串
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
