package openai

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// Client OpenAI API客户端 - 现在基于增强客户端实现
type Client struct {
	enhanced *EnhancedClient
	config   *ClientConfig
}

// ClientConfig OpenAI客户端配置
type ClientConfig struct {
	APIKey     string
	Model      string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 初始重试延迟
}

// NewClientConfig 创建新的OpenAI客户端配置
func NewClientConfig(apiKey, model, baseURL string, timeout int) *ClientConfig {
	return &ClientConfig{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		Timeout:    time.Duration(timeout) * time.Second,
		MaxRetries: 3,               // 默认重试3次
		RetryDelay: 1 * time.Second, // 默认初始延迟1秒
	}
}

// NewClient 创建新的OpenAI客户端（现在使用增强版本）
func NewClient(config *ClientConfig) *Client {
	// 创建增强客户端配置
	enhancedConfig := &EnhancedClientConfig{
		Pool: &PoolConfig{
			MaxConcurrent: 10,   // 默认最大并发数
			RateLimit:     5.0,  // 默认每秒5个请求
			BurstLimit:    15.0, // 默认突发15个请求
			ClientConfigs: []*ClientConfig{config},
		},
		Context: &ContextConfig{
			DefaultTimeout: config.Timeout,
			MaxTimeout:     config.Timeout * 3, // 最大超时为默认的3倍
		},
	}

	// 创建增强客户端
	enhanced, err := NewEnhancedClient(enhancedConfig)
	if err != nil {
		logrus.Errorf("创建增强OpenAI客户端失败: %v", err)
		// 降级处理，但这种情况不应该发生
		return nil
	}

	return &Client{
		enhanced: enhanced,
		config:   config,
	}
}

// CreateChatCompletion 创建聊天完成（兼容原接口）
func (c *Client) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c.enhanced == nil {
		return nil, fmt.Errorf("增强客户端未初始化")
	}

	// 使用增强客户端处理请求，默认任务类型为"general"
	return c.enhanced.CreateChatCompletion(ctx, req, "general")
}

// CreateChatCompletionWithTaskType 创建聊天完成（带任务类型）
func (c *Client) CreateChatCompletionWithTaskType(ctx context.Context, req *ChatCompletionRequest, taskType string) (*ChatCompletionResponse, error) {
	if c.enhanced == nil {
		return nil, fmt.Errorf("增强客户端未初始化")
	}

	return c.enhanced.CreateChatCompletion(ctx, req, taskType)
}

// CreateChatCompletionWithTimeout 创建聊天完成（带自定义超时）
func (c *Client) CreateChatCompletionWithTimeout(ctx context.Context, req *ChatCompletionRequest, taskType string, timeout time.Duration) (*ChatCompletionResponse, error) {
	if c.enhanced == nil {
		return nil, fmt.Errorf("增强客户端未初始化")
	}

	return c.enhanced.CreateChatCompletionWithTimeout(ctx, req, taskType, timeout)
}

// GetDefaultModel 获取默认模型
func (c *Client) GetDefaultModel() string {
	return c.config.Model
}

// GetStats 获取客户端统计信息
func (c *Client) GetStats() map[string]interface{} {
	if c.enhanced == nil {
		return map[string]interface{}{"error": "增强客户端未初始化"}
	}
	return c.enhanced.GetAllStats()
}

// CancelLongRunningRequests 取消长时间运行的请求
func (c *Client) CancelLongRunningRequests(maxDuration time.Duration) int {
	if c.enhanced == nil {
		return 0
	}
	return c.enhanced.CancelLongRunningRequests(maxDuration)
}

// Close 关闭客户端
func (c *Client) Close() error {
	if c.enhanced == nil {
		return nil
	}
	return c.enhanced.Close()
}

// 保持向后兼容的类型定义
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature *float32                `json:"temperature,omitempty"`
	Seed        *int                    `json:"seed,omitempty"`
	MaxTokens   *int                    `json:"max_tokens,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
}
