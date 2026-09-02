// Package ai contains provider-neutral contracts shared by AI adapters and
// runtime orchestration. Provider-specific configuration and wire payloads
// stay in their respective infrastructure packages.
package ai

import (
	"context"
	"errors"
	"time"
)

type ChatCompletionMessage struct {
	Role         string                      `json:"role"`
	Content      string                      `json:"content"`
	MultiContent []ChatCompletionContentPart `json:"multi_content,omitempty"`
}

type ChatCompletionContentPart struct {
	Type     string                          `json:"type"`
	Text     string                          `json:"text,omitempty"`
	ImageURL *ChatCompletionContentPartImage `json:"image_url,omitempty"`
}

type ChatCompletionContentPartImage struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

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

type ChatCompletionChoice struct {
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
	Index        int                   `json:"index"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Created int64                  `json:"created"`
	Usage   Usage                  `json:"usage"`
}

type TextChatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
	Generate(ctx context.Context, prompt string) (string, error)
	GetDefaultModel() string
}

type ChatCompleter interface {
	TextChatCompleter
	AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)
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
	Model            string
	Prompt           string
	Image            []byte
	ImageContentType string
	ImageURL         string
	ImageURLs        []string
	Mask             []byte
	Size             string
	Quality          string
	ResponseFormat   string
	N                int
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
	JobID             string         `json:"job_id,omitempty"`
	RequestID         string         `json:"request_id,omitempty"`
	Provider          string         `json:"provider,omitempty"`
	Status            string         `json:"status,omitempty"`
	RawSubmitResponse string         `json:"raw_submit_response,omitempty"`
	AcceptedAt        time.Time      `json:"accepted_at,omitempty"`
	Response          *ImageResponse `json:"response,omitempty"`
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
	SubmitImageEdit(ctx context.Context, req *ImageEditRequest) (*ImageAsyncSubmitResponse, error)
	QueryImageGeneration(ctx context.Context, jobID string) (*ImageAsyncQueryResponse, error)
}

// ImageRouteSelection binds an image call to an already resolved credential
// configuration. It contains no provider-specific client or configuration
// representation.
type ImageRouteSelection struct {
	CredentialReference  string
	ConfigurationVersion string
}

// RouteBoundImageGenerator executes an image edit only on the exact route
// selected before dispatch.
type RouteBoundImageGenerator interface {
	ImageGenerator
	EditImageWithRoute(context.Context, *ImageEditRequest, ImageRouteSelection) (*ImageResponse, error)
}
