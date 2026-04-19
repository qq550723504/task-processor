package productimage

import (
	"context"
	"testing"

	"task-processor/internal/prompt"
	productenrich "task-processor/internal/productenrich"
)

type reviewClientStub struct {
	lastPrompt string
}

func (s *reviewClientStub) Generate(_ context.Context, prompt string) (string, error) {
	s.lastPrompt = prompt
	return `{"needs_review":false,"confidence":0.75}`, nil
}

func (s *reviewClientStub) AnalyzeImage(_ context.Context, _ string, prompt string) (string, error) {
	s.lastPrompt = prompt
	return `{"needs_review":false,"confidence":0.75}`, nil
}

type reviewManagerStub struct {
	client productenrich.LLMClient
}

func (s *reviewManagerStub) GetClient(_ string) (productenrich.LLMClient, error) {
	return s.client, nil
}

func (s *reviewManagerStub) GetDefaultClient() productenrich.LLMClient {
	return s.client
}

func TestReviewModelUsesRegistryPromptWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageReviewDefault: "Registry review prompt {{.product_type}} / {{.title}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	client := &reviewClientStub{}
	model, err := NewLLMReviewModel(&reviewManagerStub{client: client})
	if err != nil {
		t.Fatalf("NewLLMReviewModel() error = %v", err)
	}

	_, err = model.Review(context.Background(), &ReviewModelRequest{
		Context: &ProductContext{
			ProductType: "sneaker",
			Title:       "Red running shoe",
		},
	})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}

	if client.lastPrompt != "Registry review prompt sneaker / Red running shoe" {
		t.Fatalf("prompt = %q", client.lastPrompt)
	}
}
