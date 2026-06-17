package attribute

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type stubFallbackChatCompleter struct {
	content string
	err     error
}

func (s stubFallbackChatCompleter) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Content: s.content},
		}},
	}, s.err
}

func (s stubFallbackChatCompleter) Generate(context.Context, string) (string, error) {
	return s.content, s.err
}

func (s stubFallbackChatCompleter) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

func (s stubFallbackChatCompleter) GetDefaultModel() string {
	return "test-model"
}

func TestLLMPlatformValueFallbackResolver_ResolvePlatformValueParsesJSON(t *testing.T) {
	resolver := NewLLMPlatformValueFallbackResolver(stubFallbackChatCompleter{
		content: "```json\n{\"resolved_value\":\"M\",\"confidence\":0.91,\"reason\":\"Medium best matches M\"}\n```",
	})

	result, err := resolver.ResolvePlatformValue(context.Background(), &PlatformValueFallbackRequest{
		AttrID:         87,
		Domain:         platformValueDomainApparelAlphaSize,
		RawValue:       "Med",
		ProductTitle:   "Women's Casual Blouse",
		PlatformValues: []string{"S", "M", "L"},
	})
	if err != nil {
		t.Fatalf("ResolvePlatformValue() error = %v", err)
	}
	if result == nil {
		t.Fatalf("ResolvePlatformValue() result is nil")
	}
	if result.ResolvedValue != "M" {
		t.Fatalf("resolved value = %q, want M", result.ResolvedValue)
	}
}
