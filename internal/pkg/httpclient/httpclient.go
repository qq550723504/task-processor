// Package httpclient 提供带重试和日志的 HTTP 客户端
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Config HTTP 客户端配置
type Config struct {
	Timeout       time.Duration
	MaxRetries    int
	RetryDelay    time.Duration
	EnableLogging bool
	SkipTLSVerify bool
}

// Client 增强的 HTTP 客户端
type Client struct {
	client *http.Client
	config Config
	logger *logrus.Logger
}

// New 创建 HTTP 客户端
func New(config Config, logger *logrus.Logger) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}

	return &Client{
		client: &http.Client{Timeout: config.Timeout},
		config: config,
		logger: logger,
	}
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryDelay:    time.Second,
		EnableLogging: true,
		SkipTLSVerify: false,
	}
}

// Do 执行 HTTP 请求（带重试）
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
			if c.config.EnableLogging {
				c.logger.Warnf("HTTP请求重试 %d/%d: %s %s", attempt, c.config.MaxRetries, req.Method, req.URL.String())
			}
		}

		start := time.Now()
		resp, err := c.client.Do(req.WithContext(ctx))
		latency := time.Since(start)

		if c.config.EnableLogging {
			if err != nil {
				c.logger.Errorf("HTTP请求失败: %s %s, 耗时: %v, 错误: %v", req.Method, req.URL.String(), latency, err)
			} else {
				c.logger.Infof("HTTP请求成功: %s %s, 状态码: %d, 耗时: %v", req.Method, req.URL.String(), resp.StatusCode, latency)
			}
		}

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < c.config.MaxRetries {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("服务器错误: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("HTTP请求失败，已重试 %d 次: %w", c.config.MaxRetries, lastErr)
}

// NewSimple 创建简单的 http.Client（30s 超时）
func NewSimple() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// NewSimpleWithTimeout 创建指定超时的 http.Client
func NewSimpleWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// NewWithTransport 创建带自定义 Transport 的 http.Client
func NewWithTransport(timeout time.Duration, transport *http.Transport) *http.Client {
	return &http.Client{Timeout: timeout, Transport: transport}
}
