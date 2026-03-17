// Package openai 提供OpenAI API客户端功能
package openai

import (
	"context"
	"fmt"
	"task-processor/internal/pkg/ptr"
	"time"

	"github.com/sirupsen/logrus"
)

// Client OpenAI API客户端 - 简化版本，专注于基本功能
type Client struct {
	pool   *RequestPool
	config *ClientConfig
	logger *logrus.Entry
}

// NewClient 创建新的OpenAI客户端
func NewClient(config *ClientConfig) *Client {
	// 创建请求池配置
	poolConfig := &PoolConfig{
		MaxConcurrent: 10,
		RateLimit:     5.0,
		BurstLimit:    15.0,
		ClientConfigs: []*ClientConfig{config},
	}

	// 创建请求池
	pool, err := NewRequestPool(poolConfig)
	if err != nil {
		logrus.Errorf("创建OpenAI请求池失败: %v", err)
		return nil
	}

	return &Client{
		pool:   pool,
		config: config,
		logger: logrus.WithField("component", "OpenAIClient"),
	}
}

// CreateChatCompletion 创建聊天完成
func (c *Client) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c.pool == nil {
		return nil, fmt.Errorf("请求池未初始化")
	}

	// 设置默认模型
	if req.Model == "" {
		req.Model = c.config.Model
	}

	return c.pool.CreateChatCompletion(ctx, req)
}

// CreateChatCompletionWithTimeout 创建带自定义超时的聊天完成
func (c *Client) CreateChatCompletionWithTimeout(ctx context.Context, req *ChatCompletionRequest, timeout time.Duration) (*ChatCompletionResponse, error) {
	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.CreateChatCompletion(timeoutCtx, req)
}

// GetDefaultModel 获取默认模型
func (c *Client) GetDefaultModel() string {
	return c.config.Model
}

// GetStats 获取客户端统计信息
func (c *Client) GetStats() map[string]any {
	if c.pool == nil {
		return map[string]any{"error": "请求池未初始化"}
	}
	return c.pool.GetStats()
}

// Close 关闭客户端
func (c *Client) Close() error {
	if c.pool == nil {
		return nil
	}
	return c.pool.Close()
}

// Generate 简单的文本生成（将 prompt 转换为 ChatCompletionRequest）
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	req := &ChatCompletionRequest{
		Messages: []ChatCompletionMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   ptr.IntPtr(1000),
		Temperature: ptr.Float32Ptr(0.7),
	}

	resp, err := c.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
