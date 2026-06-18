package sheinadapter

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type stubMultimodalChatClient struct {
	lastReq *openaiclient.ChatCompletionRequest
}

func (s *stubMultimodalChatClient) CreateChatCompletion(_ context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	s.lastReq = req
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Content: `{"title":"Optimized","description":"Optimized description"}`},
		}},
	}, nil
}

func (s *stubMultimodalChatClient) GetDefaultModel() string {
	return "test-model"
}

func TestMultimodalTextGeneratorBuildsOpenAIImageRequest(t *testing.T) {
	client := &stubMultimodalChatClient{}
	generator := multimodalTextGenerator{client: client}

	content, err := generator.GenerateMultimodal(context.Background(), "system prompt", "user prompt", []string{"https://example.com/main.jpg"})
	if err != nil {
		t.Fatalf("GenerateMultimodal returned error: %v", err)
	}
	if content == "" {
		t.Fatal("content is empty")
	}
	if client.lastReq == nil || len(client.lastReq.Messages) != 2 {
		t.Fatalf("request = %+v, want system and user messages", client.lastReq)
	}
	if got := client.lastReq.Messages[0].Content; got != "system prompt" {
		t.Fatalf("system prompt = %q", got)
	}
	parts := client.lastReq.Messages[1].MultiContent
	if len(parts) != 2 {
		t.Fatalf("multi-content parts = %+v, want text + image", parts)
	}
	if parts[0].Type != "text" || parts[0].Text != "user prompt" {
		t.Fatalf("text part = %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://example.com/main.jpg" {
		t.Fatalf("image part = %+v", parts[1])
	}
}
