package shein

import "context"

type TextGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type MultimodalTextGenerator interface {
	GenerateMultimodal(ctx context.Context, systemPrompt string, userPrompt string, imageURLs []string) (string, error)
}
