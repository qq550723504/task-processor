package shein

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
)

type stubTitleAIClient struct {
	response string
}

func (s stubTitleAIClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Content: s.response},
		}},
	}, nil
}

func (s stubTitleAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	return s.response, nil
}

func (s stubTitleAIClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", nil
}

func (s stubTitleAIClient) GetDefaultModel() string {
	return "stub"
}

func TestBuildSheinListingCopyKeepsStructuredEnglishTitle(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"product_english_name": {Value: "Flannel Non-slip Floor Mat"},
		},
	}

	copy := buildSheinListingCopy(canonical, canonical.Title, nil)
	if copy.Title != "Flannel Non-slip Floor Mat" {
		t.Fatalf("title = %q, want clean structured title", copy.Title)
	}
	if copy.TitleDiagnostics == nil || copy.TitleDiagnostics.PromptContaminated {
		t.Fatalf("title diagnostics = %+v, want uncontaminated", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopySanitizesPromptLikeTitleWithRules(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"product_english_name": {Value: "Flannel non-slip floor mat - Please design an image that can be printed on my non-slip floor mat. The image should include suitable English text and graphics, and the graphics and text should have a 3D visual effect. Please ensure it does not infringe on copyright. 3000 pixels * 2"},
		},
	}

	copy := buildSheinListingCopy(canonical, canonical.Title, nil)
	if copy.Title != "Flannel non-slip floor mat" {
		t.Fatalf("title = %q, want sanitized base title", copy.Title)
	}
	if copy.SKCTitleBase != "Flannel non-slip floor mat" {
		t.Fatalf("skc base title = %q, want sanitized short title", copy.SKCTitleBase)
	}
	if copy.TitleDiagnostics == nil || !copy.TitleDiagnostics.PromptContaminated || copy.TitleDiagnostics.Source != "prompt_extracted_rule" {
		t.Fatalf("title diagnostics = %+v, want prompt_extracted_rule contamination", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopyUsesLLMWhenRuleExtractionCannotRecover(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"product_english_name": {Value: "Please design an image for my floor mat with floral artwork and inspirational text, 3000 pixels"},
		},
	}

	copy := buildSheinListingCopy(canonical, canonical.Title, stubTitleAIClient{
		response: `{"title":"Flannel Floral Floor Mat"}`,
	})
	if copy.Title != "Flannel Floral Floor Mat" {
		t.Fatalf("title = %q, want llm extracted title", copy.Title)
	}
	if copy.TitleDiagnostics == nil || copy.TitleDiagnostics.Source != "prompt_extracted_llm" {
		t.Fatalf("title diagnostics = %+v, want prompt_extracted_llm", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopyFallsBackWhenLLMReturnsPromptLikeTitle(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"product_english_name": {Value: "Please design an image for my floor mat with floral artwork and inspirational text, 3000 pixels"},
			"material":             {Value: "polyester"},
		},
	}

	copy := buildSheinListingCopy(canonical, canonical.Title, stubTitleAIClient{
		response: `{"title":"Please design a floral image for this floor mat with 3D graphics"}`,
	})
	if copy.Title == "" || copy.Title == canonical.Attributes["product_english_name"].Value {
		t.Fatalf("title = %q, want non-empty fallback title", copy.Title)
	}
	if isPromptLikeTitle(copy.Title) {
		t.Fatalf("title = %q, want non prompt-like fallback", copy.Title)
	}
	if copy.TitleDiagnostics == nil || !copy.TitleDiagnostics.PromptContaminated {
		t.Fatalf("title diagnostics = %+v, want contamination note", copy.TitleDiagnostics)
	}
}
