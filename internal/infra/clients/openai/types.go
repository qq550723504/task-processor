// Package openai 提供OpenAI API客户端功能
package openai

import (
	"context"
	"errors"
	"time"
)

// ChatCompletionMessage 聊天完成消息
type ChatCompletionMessage struct {
	Role         string                      `json:"role"`
	Content      string                      `json:"content"`
	MultiContent []ChatCompletionContentPart `json:"multi_content,omitempty"`
}

// ChatCompletionContentPart 多模态消息内容块
type ChatCompletionContentPart struct {
	Type     string                          `json:"type"` // "text" 或 "image_url"
	Text     string                          `json:"text,omitempty"`
	ImageURL *ChatCompletionContentPartImage `json:"image_url,omitempty"`
}

// ChatCompletionContentPartImage 图片内容
type ChatCompletionContentPartImage struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Temperature    *float32                `json:"temperature,omitempty"`
	Seed           *int                    `json:"seed,omitempty"`
	MaxTokens      *int                    `json:"max_tokens,omitempty"`
	ResponseFormat string                  `json:"response_format,omitempty"`
	Model          string                  `json:"model"`
	Messages       []ChatCompletionMessage `json:"messages"`
	Timeout        *time.Duration          `json:"-"`
	MaxRetries     *int                    `json:"-"`
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
	APIStyle   string        `json:"api_style,omitempty"`
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

// ChatCompleter 是业务层使用 AI 客户端的最小接口。
// 所有 Handler 应依赖此接口而非具体的 *Client，以便测试和替换实现。
type ChatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
	Generate(ctx context.Context, prompt string) (string, error)
	AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)
	GetDefaultModel() string
}

type ImageGenerateRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	N              int    `json:"n,omitempty"`
}

type ImageEditRequest struct {
	Model          string
	Prompt         string
	Image          []byte
	ImageURL       string
	ImageURLs      []string
	Mask           []byte
	Size           string
	Quality        string
	ResponseFormat string
	N              int
}

type ImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageResponse struct {
	Created       int64       `json:"created"`
	Data          []ImageData `json:"data"`
	Usage         Usage       `json:"usage"`
	RequestID     string      `json:"request_id,omitempty"`
	UpstreamJobID string      `json:"upstream_job_id,omitempty"`
	RawResponse   string      `json:"raw_response,omitempty"`
}

var ErrAsyncImageGenerationNotSupported = errors.New("async image generation is not supported")

type ImageAsyncSubmitResponse struct {
	JobID             string    `json:"job_id,omitempty"`
	RequestID         string    `json:"request_id,omitempty"`
	Provider          string    `json:"provider,omitempty"`
	RawSubmitResponse string    `json:"raw_submit_response,omitempty"`
	AcceptedAt        time.Time `json:"accepted_at,omitempty"`
}

type ImageAsyncQueryResponse struct {
	JobID             string      `json:"job_id,omitempty"`
	RequestID         string      `json:"request_id,omitempty"`
	Provider          string      `json:"provider,omitempty"`
	Status            string      `json:"status,omitempty"`
	RawResultResponse string      `json:"raw_result_response,omitempty"`
	Error             string      `json:"error,omitempty"`
	Usage             Usage       `json:"usage"`
	Data              []ImageData `json:"data,omitempty"`
}

type ImageGenerator interface {
	GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error)
	EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error)
	GetDefaultModel() string
	SupportsAsyncImageGeneration() bool
	SubmitImageGeneration(ctx context.Context, req *ImageGenerateRequest) (*ImageAsyncSubmitResponse, error)
	QueryImageGeneration(ctx context.Context, jobID string) (*ImageAsyncQueryResponse, error)
}
