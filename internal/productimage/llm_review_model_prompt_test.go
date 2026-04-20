package productimage

import (
	"context"
	"testing"

	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/prompt"
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

func TestBuildReviewResolvedPromptFallsBackWhenRegistryMissing(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = nil
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildReviewResolvedPrompt(&ReviewModelRequest{
		Context: &ProductContext{
			ProductType: "sneaker",
			Title:       "Red running shoe",
		},
	}, `{"quality":"ok"}`)

	if resolved.Key != prompt.KProductImageReviewDefault {
		t.Fatalf("key = %q", resolved.Key)
	}
	if resolved.Source != "fallback" || resolved.Version != "default" {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Text == "" {
		t.Fatal("resolved.Text is empty")
	}
}

func TestBuildReviewResolvedPromptUsesRegistryMetadata(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageReviewDefault: "Registry review prompt {{.product_type}} / {{.title}} / {{.summary_json}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildReviewResolvedPrompt(&ReviewModelRequest{
		Context: &ProductContext{
			ProductType: "sneaker",
			Title:       "Red running shoe",
		},
	}, `{"quality":"ok"}`)

	if resolved.Key != prompt.KProductImageReviewDefault {
		t.Fatalf("key = %q", resolved.Key)
	}
	if resolved.Source != "registry" || resolved.Version != "default" {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Text != `Registry review prompt sneaker / Red running shoe / {"quality":"ok"}` {
		t.Fatalf("prompt = %q", resolved.Text)
	}
}
