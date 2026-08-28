package sheinadapter

import (
	"context"

	"task-processor/internal/ai"
	sheinpub "task-processor/internal/publishing/shein"
)

type ChatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error)
	GetDefaultModel() string
}

type multimodalTextGenerator struct {
	client ChatCompleter
}

func NewReviewContentOptimizer(client ChatCompleter) sheinpub.ReviewContentOptimizer {
	if client == nil {
		return nil
	}
	return sheinpub.NewReviewContentOptimizer(multimodalTextGenerator{client: client})
}

func (g multimodalTextGenerator) GenerateMultimodal(ctx context.Context, systemPrompt string, userPrompt string, imageURLs []string) (string, error) {
	temperature := float32(0.7)
	messages := []ai.ChatCompletionMessage{{
		Role:    "system",
		Content: systemPrompt,
	}}
	if len(imageURLs) == 0 {
		messages = append(messages, ai.ChatCompletionMessage{
			Role:    "user",
			Content: userPrompt,
		})
	} else {
		parts := make([]ai.ChatCompletionContentPart, 0, 1+len(imageURLs))
		parts = append(parts, ai.ChatCompletionContentPart{
			Type: "text",
			Text: userPrompt,
		})
		for _, imageURL := range imageURLs {
			parts = append(parts, ai.ChatCompletionContentPart{
				Type: "image_url",
				ImageURL: &ai.ChatCompletionContentPartImage{
					URL:    imageURL,
					Detail: "auto",
				},
			})
		}
		messages = append(messages, ai.ChatCompletionMessage{
			Role:         "user",
			MultiContent: parts,
		})
	}
	resp, err := g.client.CreateChatCompletion(ctx, &ai.ChatCompletionRequest{
		Model:       g.client.GetDefaultModel(),
		Temperature: &temperature,
		Messages:    messages,
	})
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}
