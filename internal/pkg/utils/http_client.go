// Package utils 提供工具方法
package utils

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPClientConfig HTTP客户端配置
type HTTPClientConfig struct {
	Timeout       time.Duration
	MaxRetries    int
	RetryDelay    time.Duration
	EnableLogging bool
	SkipTLSVerify bool
}

// HTTPClient 增强的HTTP客户端
type HTTPClient struct {
	client *http.Client
	config HTTPClientConfig
	logger *logrus.Logger
}

// NewHTTPClient 创建HTTP客户端
func NewHTTPClient(config HTTPClientConfig, logger *logrus.Logger) *HTTPClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config: config,
		logger: logger,
	}
}

// Do 执行HTTP请求（带重试）
func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 等待重试延迟
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}

			if c.config.EnableLogging {
				c.logger.Warnf("HTTP请求重试 %d/%d: %s %s",
					attempt, c.config.MaxRetries, req.Method, req.URL.String())
			}
		}

		// 记录请求开始
		start := time.Now()
		if c.config.EnableLogging {
			c.logger.Infof("HTTP请求开始: %s %s", req.Method, req.URL.String())
		}

		// 执行请求
		resp, err := c.client.Do(req.WithContext(ctx))
		latency := time.Since(start)

		// 记录请求结果
		if c.config.EnableLogging {
			if err != nil {
				c.logger.Errorf("HTTP请求失败: %s %s, 耗时: %v, 错误: %v",
					req.Method, req.URL.String(), latency, err)
			} else {
				c.logger.Infof("HTTP请求成功: %s %s, 状态码: %d, 耗时: %v",
					req.Method, req.URL.String(), resp.StatusCode, latency)
			}
		}

		if err != nil {
			lastErr = err
			continue
		}

		// 检查是否需要重试（5xx错误）
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < c.config.MaxRetries {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("服务器错误: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("HTTP请求失败，已重试 %d 次: %w", c.config.MaxRetries, lastErr)
}

// GetDefaultHTTPClientConfig 获取默认HTTP客户端配置
func GetDefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryDelay:    time.Second,
		EnableLogging: true,
		SkipTLSVerify: false,
	}
}

// CreateSimpleHTTPClient 创建简单的HTTP客户端（用于替换重复的http.Client创建）
func CreateSimpleHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// CreateSimpleHTTPClientWithTimeout 创建指定超时的简单HTTP客户端
func CreateSimpleHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// CreateHTTPClientWithTransport 创建带自定义Transport的HTTP客户端
func CreateHTTPClientWithTransport(timeout time.Duration, transport *http.Transport) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
