package listingkit

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/prompt"
)

func TestParseStudioDesignSiblingThemesParsesPromptObject(t *testing.T) {
	themes, err := parseStudioDesignSiblingThemes(`{"prompts":["bold vintage badge","dense collage badge","hero emblem badge"]}`, 3)
	if err != nil {
		t.Fatalf("parseStudioDesignSiblingThemes() error = %v", err)
	}
	if strings.Join(themes, "|") != "bold vintage badge|dense collage badge|hero emblem badge" {
		t.Fatalf("themes = %#v", themes)
	}
}

func TestGenerateStudioDesignSiblingThemesFallsBackWhenLLMResponseInvalid(t *testing.T) {
	svc := &service{
		studioPromptDiversifier: &stubStudioChatCompleter{
			generateText: "not-json",
		},
	}
	req := &StudioDesignRequest{
		Prompt:             "retro dog college badge",
		VariationIntensity: studioVariationStrong,
	}

	themes, err := svc.generateStudioDesignSiblingThemes(context.Background(), req, 3)
	if err == nil {
		t.Fatalf("expected error when llm returns invalid payload")
	}
	if len(themes) != 3 {
		t.Fatalf("themes count = %d, want 3", len(themes))
	}
	for _, theme := range themes {
		if theme != req.Prompt {
			t.Fatalf("fallback theme = %q, want %q", theme, req.Prompt)
		}
	}
}

func TestGenerateStudioDesignSiblingThemesUsesLLMOutput(t *testing.T) {
	svc := &service{
		studioPromptDiversifier: &stubStudioChatCompleter{
			generateText: `{"prompts":["vintage varsity crest with centered mascot","vintage varsity crest with repeating border icons"]}`,
		},
	}
	req := &StudioDesignRequest{
		Prompt:             "vintage varsity crest mascot",
		VariationIntensity: studioVariationMedium,
	}

	themes, err := svc.generateStudioDesignSiblingThemes(context.Background(), req, 2)
	if err != nil {
		t.Fatalf("generateStudioDesignSiblingThemes() error = %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("themes count = %d, want 2", len(themes))
	}
	if themes[0] == themes[1] {
		t.Fatalf("themes should differ: %#v", themes)
	}
}

func TestStudioDesignPromptRendersWithoutTransparencyInstructions(t *testing.T) {
	registry := prompt.NewRegistry(logrus.NewEntry(logrus.New()))
	if err := registry.Init(context.Background(), filepath.Join("..", "..", "prompts"), false); err != nil {
		t.Fatalf("init prompt registry: %v", err)
	}
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = registry
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	text := buildStudioDesignPrompt(&StudioDesignRequest{
		Prompt:                    "national flag of the United States",
		PrintableWidth:            1000,
		PrintableHeight:           600,
		ProductReferenceImageURLs: []string{"https://example.com/black-shirt.jpg", "https://example.com/white-shirt.jpg"},
	})
	lower := strings.ToLower(text)

	for _, forbidden := range []string{
		"{{",
		"transparent background",
		"fully transparent",
		"alpha channel",
		"checkerboard",
		"simulate transparency",
		"apparel",
		"garment",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("prompt contains forbidden %q:\n%s", forbidden, text)
		}
	}
	for _, required := range []string{
		"customized-product",
		"target print area: 1000 by 600 pixels",
		"product color variants",
		"generate only the flat artwork/design",
	} {
		if !strings.Contains(lower, required) {
			t.Fatalf("prompt missing %q:\n%s", required, text)
		}
	}
}

func TestStudioDesignReferenceImageURLsDeduplicatesAndCaps(t *testing.T) {
	got := studioDesignReferenceImageURLs([]string{
		"",
		" https://example.com/a.png ",
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
		"https://example.com/d.png",
		"https://example.com/e.png",
		"https://example.com/f.png",
	})
	want := []string{
		"https://example.com/a.png",
		"https://example.com/b.png",
		"https://example.com/c.png",
		"https://example.com/d.png",
		"https://example.com/e.png",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("references = %#v, want %#v", got, want)
	}
}

func TestStudioDesignPromptIncludesTransparencyInstructionsWhenRequested(t *testing.T) {
	text := buildStudioDesignPrompt(&StudioDesignRequest{
		Prompt:                "minimal dog badge",
		TransparentBackground: true,
	})
	lower := strings.ToLower(text)
	for _, required := range []string{
		"true transparent background",
		"alpha channel",
		"do not simulate transparency",
	} {
		if !strings.Contains(lower, required) {
			t.Fatalf("prompt missing %q:\n%s", required, text)
		}
	}
}

func TestResolveStudioDesignImageModelUsesGPTForTransparency(t *testing.T) {
	got := resolveStudioDesignImageModel(&StudioDesignRequest{
		ImageModel:            "nano-banana-fast",
		TransparentBackground: true,
	}, "fallback-model")
	if got != studioDesignTransparentModel {
		t.Fatalf("model = %q, want %q", got, studioDesignTransparentModel)
	}

	got = resolveStudioDesignImageModel(&StudioDesignRequest{
		ImageModel: "custom-model",
	}, "fallback-model")
	if got != "custom-model" {
		t.Fatalf("model = %q, want custom-model", got)
	}
}

func TestGenerateStudioDesignImageFallsBackWhenMultiReferenceEditFails(t *testing.T) {
	generator := &stubStudioImageGenerator{
		editErr: errors.New("provider rejected references"),
		generateResponse: &openaiclient.ImageResponse{Data: []openaiclient.ImageData{{
			B64JSON: "aW1hZ2U=",
		}}},
	}
	svc := &service{studioImageGenerator: generator}

	response, err := svc.generateStudioDesignImage(context.Background(), "test-model", "prompt", "1024x1024", []string{
		"https://example.com/black.png",
		"https://example.com/white.png",
	})
	if err != nil {
		t.Fatalf("generateStudioDesignImage() error = %v", err)
	}
	if response == nil || len(response.Data) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if generator.editCalls != 2 {
		t.Fatalf("editCalls = %d, want 2", generator.editCalls)
	}
	if generator.generateCalls != 1 {
		t.Fatalf("generateCalls = %d, want 1", generator.generateCalls)
	}
}

type stubStudioImageGenerator struct {
	editErr          error
	editCalls        int
	generateCalls    int
	generateResponse *openaiclient.ImageResponse
}

type stubStudioChatCompleter struct {
	generateText string
	generateErr  error
}

func (s *stubStudioChatCompleter) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubStudioChatCompleter) Generate(context.Context, string) (string, error) {
	return s.generateText, s.generateErr
}

func (s *stubStudioChatCompleter) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *stubStudioChatCompleter) GetDefaultModel() string {
	return "test-model"
}

func (s *stubStudioImageGenerator) GenerateImage(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	s.generateCalls++
	return s.generateResponse, nil
}

func (s *stubStudioImageGenerator) EditImage(context.Context, *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	s.editCalls++
	return nil, s.editErr
}

func (s *stubStudioImageGenerator) GetDefaultModel() string {
	return "test-model"
}

func TestStudioProductImagePromptRendersManagedTemplate(t *testing.T) {
	registry := prompt.NewRegistry(logrus.NewEntry(logrus.New()))
	if err := registry.Init(context.Background(), filepath.Join("..", "..", "prompts"), false); err != nil {
		t.Fatalf("init prompt registry: %v", err)
	}
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = registry
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	text := buildStudioProductImagePrompt(&StudioProductImageRequest{
		Prompt:       "space dog",
		ProductName:  "Baseball cap",
		CategoryPath: []string{"Home", "Decor"},
		StyleName:    "Style A",
		CustomPrompt: "bright studio light",
		ImagePrompts: []StudioProductImagePrompt{
			{Role: "main", Prompt: "front-facing white background hero shot"},
			{Role: "detail", Prompt: "stitched seam close-up"},
		},
	}, defaultStudioProductImageRoles[0], 1, 3)

	if strings.Contains(text, "{{") {
		t.Fatalf("prompt contains unrendered template action:\n%s", text)
	}
	if !strings.Contains(text, "Image 1 of 3.") || !strings.Contains(text, "Theme prompt: space dog") {
		t.Fatalf("prompt did not render expected fields:\n%s", text)
	}
	if !strings.Contains(text, "Global: bright studio light") ||
		!strings.Contains(text, "Main image: front-facing white background hero shot") ||
		strings.Contains(text, "stitched seam close-up") {
		t.Fatalf("prompt did not apply role-specific instructions correctly:\n%s", text)
	}
	for _, required := range []string{
		"first input image is the approved pod artwork",
		"preserve the approved artwork's exact visual identity",
		"do not redesign, reinterpret",
		"same design across every generated image",
	} {
		if !strings.Contains(strings.ToLower(text), required) {
			t.Fatalf("prompt missing %q:\n%s", required, text)
		}
	}
}
