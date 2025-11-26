package openai

import (
	"context"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// Client OpenAI API客户端
type Client struct {
	client *openai.Client
	config *ClientConfig
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

// NewClient 创建新的OpenAI客户端
func NewClient(config *ClientConfig) *Client {
	// 创建OpenAI客户端配置
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	// 创建OpenAI客户端
	client := openai.NewClientWithConfig(clientConfig)

	return &Client{
		client: client,
		config: config,
	}
}

// ChatCompletionMessage 聊天完成消息
type ChatCompletionMessage = openai.ChatCompletionMessage

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature *float32                `json:"temperature,omitempty"`
	Seed        *int                    `json:"seed,omitempty"`
	MaxTokens   *int                    `json:"max_tokens,omitempty"`
}

// ChatCompletionResponse 聊天完成响应
type ChatCompletionResponse struct {
	ID      string                        `json:"id"`
	Object  string                        `json:"object"`
	Created int64                         `json:"created"`
	Model   string                        `json:"model"`
	Choices []openai.ChatCompletionChoice `json:"choices"`
	Usage   openai.Usage                  `json:"usage"`
}

// CreateChatCompletion 创建聊天完成（带重试机制）
func (c *Client) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算指数退避延迟时间
			delay := c.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			logrus.Warnf("OpenAI API调用失败，第%d次重试，等待%v后重试: %v", attempt, delay, lastErr)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			}
		}

		// 设置超时
		timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)

		// 转换请求参数
		openaiReq := openai.ChatCompletionRequest{
			Model:    req.Model,
			Messages: req.Messages,
		}

		// 设置可选参数
		if req.Temperature != nil {
			openaiReq.Temperature = *req.Temperature
		}

		if req.Seed != nil {
			openaiReq.Seed = req.Seed
		}

		if req.MaxTokens != nil {
			openaiReq.MaxTokens = *req.MaxTokens
		}

		// 调用OpenAI API
		resp, err := c.client.CreateChatCompletion(timeoutCtx, openaiReq)
		cancel() // 立即释放资源

		if err == nil {
			// 成功，返回响应
			if attempt > 0 {
				logrus.Infof("OpenAI API调用在第%d次重试后成功", attempt)
			}
			return &ChatCompletionResponse{
				ID:      resp.ID,
				Object:  resp.Object,
				Created: resp.Created,
				Model:   resp.Model,
				Choices: resp.Choices,
				Usage:   resp.Usage,
			}, nil
		}

		lastErr = err

		// 检查是否应该重试
		if !shouldRetry(err) {
			logrus.Warnf("OpenAI API调用失败，错误不可重试: %v", err)
			break
		}
	}

	return nil, fmt.Errorf("调用OpenAI API失败，已重试%d次: %w", c.config.MaxRetries, lastErr)
}

// shouldRetry 判断错误是否应该重试
func shouldRetry(err error) bool {
	// 可以根据具体错误类型判断是否重试
	// 例如：网络错误、超时、429限流等应该重试
	// 400错误（参数错误）、401错误（认证失败）等不应该重试
	if err == nil {
		return false
	}

	// 这里简化处理，大部分错误都重试
	// 实际项目中可以根据具体错误码判断
	return true
}

// GetDefaultModel 获取默认模型
func (c *Client) GetDefaultModel() string {
	return c.config.Model
}
