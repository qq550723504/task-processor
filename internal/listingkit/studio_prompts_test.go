package listingkit

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
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
	svc := &service{studioDeps: studioDependencies{promptDiversifier: &stubStudioChatCompleter{
		generateText: "not-json",
	}},
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
	svc := &service{studioDeps: studioDependencies{promptDiversifier: &stubStudioChatCompleter{
		generateText: `{"prompts":["vintage varsity crest with centered mascot","vintage varsity crest with repeating border icons"]}`,
	}},
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
		generateResponse: &AIImageResponse{Data: []AIImageData{{
			B64JSON: "aW1hZ2U=",
		}}},
	}
	svc := &service{studioDeps: studioDependencies{imageGenerator: generator}}

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
	mu               sync.Mutex
	editErr          error
	editErrs         []error
	editCalls        int
	editRequests     []*AIImageEditRequest
	asyncEditCalls   int
	asyncEditReqs    []*AIImageEditRequest
	asyncSubmit      *AIImageAsyncSubmit
	generateCalls    int
	generateErrs     []error
	generateResponse *AIImageResponse
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

func (s *stubStudioImageGenerator) GenerateImage(context.Context, *AIImageGenerateRequest) (*AIImageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generateCalls++
	if len(s.generateErrs) > 0 {
		idx := s.generateCalls - 1
		if idx < len(s.generateErrs) && s.generateErrs[idx] != nil {
			return nil, s.generateErrs[idx]
		}
	}
	return s.generateResponse, nil
}

func (s *stubStudioImageGenerator) EditImage(_ context.Context, req *AIImageEditRequest) (*AIImageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.editCalls++
	reqCopy := *req
	s.editRequests = append(s.editRequests, &reqCopy)
	if len(s.editErrs) > 0 {
		idx := s.editCalls - 1
		if idx < len(s.editErrs) {
			if s.editErrs[idx] != nil {
				return nil, s.editErrs[idx]
			}
			return s.generateResponse, nil
		}
	}
	if s.editErr != nil {
		return nil, s.editErr
	}
	return s.generateResponse, nil
}

func (s *stubStudioImageGenerator) GetDefaultModel() string {
	return "test-model"
}

func (s *stubStudioImageGenerator) SupportsAsyncImageGeneration() bool {
	return s.asyncSubmit != nil
}

func (s *stubStudioImageGenerator) SubmitImageGeneration(context.Context, *AIImageGenerateRequest) (*AIImageAsyncSubmit, error) {
	return s.asyncSubmit, nil
}

func (s *stubStudioImageGenerator) SubmitImageEdit(_ context.Context, req *AIImageEditRequest) (*AIImageAsyncSubmit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asyncEditCalls++
	reqCopy := *req
	s.asyncEditReqs = append(s.asyncEditReqs, &reqCopy)
	return s.asyncSubmit, nil
}

func (s *stubStudioImageGenerator) QueryImageGeneration(context.Context, string) (*AIImageAsyncResult, error) {
	return nil, nil
}

func TestSubmitStudioDesignsAsyncUsesEditPathWhenReferenceImagesExist(t *testing.T) {
	generator := &stubStudioImageGenerator{
		asyncSubmit: &AIImageAsyncSubmit{
			JobID:     "job-1",
			RequestID: "req-1",
			Provider:  "nanobanana",
		},
	}
	svc := &taskStudioMediaService{imageGenerator: generator}

	result, err := svc.SubmitStudioDesignsAsync(context.Background(), &StudioDesignRequest{
		Prompt:                    "retro cherries",
		Count:                     1,
		ProductReferenceImageURLs: []string{"https://example.com/black-shirt.jpg", "https://example.com/white-shirt.jpg"},
	})
	if err != nil {
		t.Fatalf("SubmitStudioDesignsAsync() error = %v", err)
	}
	if result == nil || result.JobID != "job-1" {
		t.Fatalf("result = %+v, want async submit metadata", result)
	}
	if generator.asyncEditCalls != 1 {
		t.Fatalf("asyncEditCalls = %d, want 1", generator.asyncEditCalls)
	}
	if len(generator.asyncEditReqs) != 1 || len(generator.asyncEditReqs[0].ImageURLs) != 2 {
		t.Fatalf("async edit requests = %+v, want 2 reference images", generator.asyncEditReqs)
	}
}

func TestGenerateStudioDesignsAddsWarningsForPromptFallbackAndPartialSuccess(t *testing.T) {
	generator := &stubStudioImageGenerator{
		generateErrs: []error{
			nil,
			errors.New("upstream rate limited"),
			errors.New("upstream timeout"),
			errors.New("upstream rate limited"),
			errors.New("upstream timeout"),
		},
		generateResponse: &AIImageResponse{
			Data: []AIImageData{{
				B64JSON: base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xD9}),
			}},
		},
	}
	svc := &service{studioDeps: studioDependencies{imageGenerator: generator, promptDiversifier: &stubStudioChatCompleter{
		generateText: "not-json",
	}, uploadStore: &stubImageUploadStore{}},
	}

	response, err := svc.GenerateStudioDesigns(context.Background(), &StudioDesignRequest{
		Prompt:             "retro cherries",
		Count:              3,
		VariationIntensity: studioVariationStrong,
	})
	if err != nil {
		t.Fatalf("GenerateStudioDesigns() error = %v", err)
	}
	if len(response.Images) != 1 {
		t.Fatalf("images = %d, want 1", len(response.Images))
	}
	if generator.generateCalls != 5 {
		t.Fatalf("generateCalls = %d, want 5", generator.generateCalls)
	}
	if len(response.Warnings) != 2 {
		t.Fatalf("warnings = %#v, want 2 entries", response.Warnings)
	}
	if !strings.Contains(response.Warnings[0], "已回退为基础提示词重复生成") {
		t.Fatalf("warning[0] = %q", response.Warnings[0])
	}
	if !strings.Contains(response.Warnings[1], "请求生成 3 款，实际仅成功 1 款") {
		t.Fatalf("warning[1] = %q", response.Warnings[1])
	}
	if !strings.Contains(response.Warnings[1], "upstream rate limited") {
		t.Fatalf("warning[1] = %q, want first failure reason", response.Warnings[1])
	}
}

func TestGenerateStudioDesignsRetriesFailedVariantsSequentially(t *testing.T) {
	generator := &stubStudioImageGenerator{
		generateErrs: []error{
			errors.New("transient upstream error"),
			errors.New("transient upstream error"),
			nil,
			nil,
			nil,
		},
		generateResponse: &AIImageResponse{
			Data: []AIImageData{{
				B64JSON: base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xD9}),
			}},
		},
	}
	svc := &service{studioDeps: studioDependencies{imageGenerator: generator, uploadStore: &stubImageUploadStore{}}}

	response, err := svc.GenerateStudioDesigns(context.Background(), &StudioDesignRequest{
		Prompt: "retro cherries",
		Count:  3,
	})
	if err != nil {
		t.Fatalf("GenerateStudioDesigns() error = %v", err)
	}
	if len(response.Images) != 3 {
		t.Fatalf("images = %d, want 3", len(response.Images))
	}
	if len(response.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none after successful refill", response.Warnings)
	}
	if generator.generateCalls != 5 {
		t.Fatalf("generateCalls = %d, want 5 (3 initial + 2 retries)", generator.generateCalls)
	}
}

func TestGenerateStudioDesignsCapsCountAtTen(t *testing.T) {
	generator := &stubStudioImageGenerator{
		generateResponse: &AIImageResponse{
			Data: []AIImageData{{
				B64JSON: base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xD9}),
			}},
		},
	}
	svc := &service{studioDeps: studioDependencies{imageGenerator: generator, uploadStore: &stubImageUploadStore{}}}

	response, err := svc.GenerateStudioDesigns(context.Background(), &StudioDesignRequest{
		Prompt: "retro cherries",
		Count:  12,
	})
	if err != nil {
		t.Fatalf("GenerateStudioDesigns() error = %v", err)
	}
	if len(response.Images) != 10 {
		t.Fatalf("images = %d, want 10", len(response.Images))
	}
	if generator.generateCalls != 10 {
		t.Fatalf("generateCalls = %d, want 10", generator.generateCalls)
	}
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

type stubImageUploadStore struct {
	saved []*ImageUploadInput
}

func (s *stubImageUploadStore) Save(_ context.Context, input *ImageUploadInput) (*StoredUploadedImage, error) {
	s.saved = append(s.saved, input)
	return &StoredUploadedImage{
		Key:         "compat/sanitized.jpg",
		Filename:    input.Filename,
		PublicURL:   "https://example.com/compat/sanitized.jpg",
		ContentType: input.ContentType,
		Size:        int64(len(input.Data)),
	}, nil
}

func (s *stubImageUploadStore) Open(context.Context, string) (*StoredUploadedImage, error) {
	return nil, ErrUploadedImageNotFound
}

func (s *stubImageUploadStore) Delete(context.Context, string) error {
	return ErrUploadedImageNotFound
}

func TestGenerateOneStudioProductImageRetriesWithSanitizedInputsOnFormatError(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	img.Set(1, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
	img.Set(0, 1, color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	img.Set(1, 1, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBuf.Bytes())
	}))
	defer server.Close()

	generator := &stubStudioImageGenerator{
		editErrs: []error{
			errors.New("nanobanana job failed: error (The image format is incorrect. Please check if there are any issues with the image format)"),
			errors.New("nanobanana job failed: error (The image format is incorrect. Please check if there are any issues with the image format)"),
			nil,
		},
		generateResponse: &AIImageResponse{
			Data: []AIImageData{{
				B64JSON: base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xD9}),
			}},
		},
	}
	store := &stubImageUploadStore{}
	svc := &service{studioDeps: studioDependencies{imageGenerator: generator, uploadStore: store}}

	_, err := svc.generateOneStudioProductImage(context.Background(), &StudioProductImageRequest{}, server.URL+"/input.png", "prompt")
	if err != nil {
		t.Fatalf("generateOneStudioProductImage() error = %v", err)
	}
	if generator.editCalls != 3 {
		t.Fatalf("editCalls = %d, want 3", generator.editCalls)
	}
	if len(store.saved) < 2 {
		t.Fatalf("saved images = %d, want at least 2", len(store.saved))
	}
	if got := store.saved[0].ContentType; got != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", got)
	}
	if len(generator.editRequests) < 3 || generator.editRequests[2].ImageURLs[0] != "https://example.com/compat/sanitized.jpg" {
		t.Fatalf("sanitized retry did not use uploaded jpeg URL: %#v", generator.editRequests)
	}
}
