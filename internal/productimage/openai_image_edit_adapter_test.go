package productimage

import (
	"testing"

	"task-processor/internal/ai"
)

func TestConvertOpenAIImageEditResponsePreservesURL(t *testing.T) {
	converted, err := convertOpenAIImageEditResponse(&ai.ImageResponse{
		Data: []ai.ImageData{{
			URL:           "https://cdn.example.com/generated.png",
			RevisedPrompt: "revised",
		}},
	}, nil)
	if err != nil {
		t.Fatalf("convertOpenAIImageEditResponse() error = %v", err)
	}
	if converted == nil || len(converted.Data) != 1 {
		t.Fatalf("converted response = %#v, want one image", converted)
	}
	if converted.Data[0].URL != "https://cdn.example.com/generated.png" {
		t.Fatalf("URL = %q, want generated image URL", converted.Data[0].URL)
	}
}
