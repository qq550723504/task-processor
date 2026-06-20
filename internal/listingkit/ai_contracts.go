package listingkit

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type AIChatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error)
	Generate(ctx context.Context, prompt string) (string, error)
	AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)
	GetDefaultModel() string
}

type AIImageGenerator interface {
	GenerateImage(ctx context.Context, req *AIImageGenerateRequest) (*AIImageResponse, error)
	EditImage(ctx context.Context, req *AIImageEditRequest) (*AIImageResponse, error)
	GetDefaultModel() string
}

type AIImageGenerateRequest struct {
	Model          string
	Prompt         string
	Size           string
	ResponseFormat string
	N              int
}

type AIImageEditRequest struct {
	Model          string
	Prompt         string
	ImageURL       string
	ImageURLs      []string
	Size           string
	ResponseFormat string
	N              int
}

type AIImageResponse struct {
	Data          []AIImageData
	Usage         AIUsage
	RequestID     string
	UpstreamJobID string
	RawResponse   string
}

type AIImageData struct {
	URL           string
	B64JSON       string
	RevisedPrompt string
}

type AIUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type StudioAIUsage = AIUsage
