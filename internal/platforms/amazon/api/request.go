package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"task-processor/internal/pkg/jsonutil"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// ErrAPIRateLimit API速率限制
	ErrAPIRateLimit = errors.New("API rate limit exceeded")
)

// APIError Amazon API错误
type APIError struct {
	Code    string
	Message string
	Details map[string]any
}

// Error 实现error接口
func (e *APIError) Error() string {
	return fmt.Sprintf("Amazon API错误 [%s]: %s", e.Code, e.Message)
}

// NewAPIError 创建API错误
func NewAPIError(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: make(map[string]any),
	}
}

// doRequest 执行 HTTP 请求的通用方法（带重试和速率限制）
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	// 获取对应的速率限制器
	limiter := c.rateLimits.GetLimiterForPath(path)

	// 等待速率限制
	if err := limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	// 使用断路器和重试机制执行请求
	operation := fmt.Sprintf("%s %s", method, path)

	return c.ExecuteWithRetry(ctx, operation, func(ctx context.Context) (*http.Response, error) {
		return c.doRequestInternal(ctx, method, path, body)
	})
}

// doRequestInternal 执行单次HTTP请求（内部方法）
func (c *Client) doRequestInternal(ctx context.Context, method, path string, body any) (*http.Response, error) {
	// 构建完整 URL
	url := c.buildURL(path)

	// 序列化请求体
	var bodyBytes []byte
	var bodyReader io.Reader
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
		c.logger.Infof("📋 完整请求体: %s", string(bodyBytes))
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 获取访问令牌
	accessToken, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-amz-access-token", accessToken)
	req.Header.Set("x-amz-date", time.Now().UTC().Format("20060102T150405Z"))
	req.Header.Set("User-Agent", "task-processor/1.0")

	// 如果有AWS签名器，进行签名
	if c.awsSigner != nil {
		if err := c.awsSigner.SignRequest(req, bodyBytes); err != nil {
			return nil, fmt.Errorf("AWS签名失败: %w", err)
		}
		c.logger.Debug("✅ AWS签名已应用")
	}

	c.logger.WithFields(logrus.Fields{
		"method": method,
		"url":    url,
		"signed": c.awsSigner != nil,
	}).Info("发送 API 请求")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"status": resp.StatusCode,
	}).Info("收到 API 响应")

	return resp, nil
}

// parseResponse 解析响应
func (c *Client) parseResponse(resp *http.Response, result any) error {
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	c.logger.Infof("响应体: %s", string(body))

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseError(resp.StatusCode, body)
	}

	// 解析成功响应
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("解析响应失败: %w", err)
		}
	}

	return nil
}

// parseError 解析错误响应
func (c *Client) parseError(statusCode int, body []byte) error {
	var errorResp struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details string `json:"details"`
		} `json:"errors"`
	}

	if err := jsonutil.UnmarshalBytes(body, &errorResp, ""); err != nil {
		return fmt.Errorf("API 错误 (status=%d): %s", statusCode, string(body))
	}

	if len(errorResp.Errors) > 0 {
		firstError := errorResp.Errors[0]
		return NewAPIError(firstError.Code, firstError.Message)
	}

	return fmt.Errorf("API 错误 (status=%d)", statusCode)
}

// handleRateLimit 处理速率限制
func (c *Client) handleRateLimit(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		c.logger.Warn("遇到 API 速率限制")
		return ErrAPIRateLimit
	}
	return nil
}
