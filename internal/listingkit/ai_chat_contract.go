package listingkit

import (
	"context"

	"task-processor/internal/ai"
)

type AIChatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error)
	Generate(ctx context.Context, prompt string) (string, error)
	AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)
	GetDefaultModel() string
}
