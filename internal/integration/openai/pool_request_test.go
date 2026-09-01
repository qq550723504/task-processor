package openai

import (
	"testing"

	goopenai "github.com/sashabaranov/go-openai"
)

func TestBuildOpenAIChatCompletionRequest_MapsJSONObjectResponseFormat(t *testing.T) {
	t.Parallel()

	req := &ChatCompletionRequest{
		Model:          "test-model",
		Messages:       []ChatCompletionMessage{{Role: "user", Content: "hello"}},
		ResponseFormat: "json_object",
	}

	got := buildOpenAIChatCompletionRequest(req)

	if got.ResponseFormat == nil {
		t.Fatal("ResponseFormat = nil, want json_object response format")
	}
	if got.ResponseFormat.Type != goopenai.ChatCompletionResponseFormatTypeJSONObject {
		t.Fatalf("ResponseFormat.Type = %q, want %q", got.ResponseFormat.Type, goopenai.ChatCompletionResponseFormatTypeJSONObject)
	}
}
