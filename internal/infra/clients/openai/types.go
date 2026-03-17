// Package openai 提供OpenAI API客户端功能
package openai

import "time"

// ChatCompletionMessage 聊天完成消息
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Temperature *float32                `json:"temperature,omitempty"`
	Seed        *int                    `json:"seed,omitempty"`
	MaxTokens   *int                    `json:"max_tokens,omitempty"`
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
}

// ChatCompletionChoice 聊天完成选择
type ChatCompletionChoice struct {
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
	Index        int                   `json:"index"`
}

// Usage 使用情况统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse 聊天完成响应
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Created int64                  `json:"created"`
	Usage   Usage                  `json:"usage"`
}

// ClientConfig OpenAI客户端配置
type ClientConfig struct {
	APIKey     string        `json:"api_key"`
	Model      string        `json:"model"`
	BaseURL    string        `json:"base_url"`
	Timeout    time.Duration `json:"timeout"`
	MaxRetries int           `json:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay"`
}

// NewClientConfig 创建新的OpenAI客户端配置
func NewClientConfig(apiKey, model, baseURL string, timeout int) *ClientConfig {
	return &ClientConfig{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		Timeout:    time.Duration(timeout) * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}

// PoolConfig 请求池配置
type PoolConfig struct {
	RateLimit     float64         `json:"rate_limit"`
	BurstLimit    float64         `json:"burst_limit"`
	ClientConfigs []*ClientConfig `json:"client_configs"`
	MaxConcurrent int             `json:"max_concurrent"`
}
