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
	GenerateImage(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error)
	EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error)
	GetDefaultModel() string
}
