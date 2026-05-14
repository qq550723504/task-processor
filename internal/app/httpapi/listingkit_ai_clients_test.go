package httpapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type stubListingKitClientResolver struct {
	resolved     *openaiclient.ResolvedClientConfig
	err          error
	lastName     string
	lastFallback *openaiclient.ClientConfig
}

type stubListingKitImageGenerator struct {
	lastGenerate *openaiclient.ImageGenerateRequest
	lastEdit     *openaiclient.ImageEditRequest
}

func (s *stubListingKitImageGenerator) GenerateImage(_ context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	s.lastGenerate = req
	return &openaiclient.ImageResponse{}, nil
}

func (s *stubListingKitImageGenerator) EditImage(_ context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	s.lastEdit = req
	return &openaiclient.ImageResponse{}, nil
}

func (s *stubListingKitImageGenerator) GetDefaultModel() string {
	return ""
}

func (r *stubListingKitClientResolver) ResolveClientConfig(_ context.Context, clientName string, fallback *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.lastName = clientName
	r.lastFallback = fallback
	if r.err != nil {
		return nil, r.err
	}
	return r.resolved, nil
}

func TestBuildListingKitClientFallbackStripsSensitiveFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.APIKey = "shared-key"
	cfg.OpenAI.BaseURL = "https://default.example.test/v1"
	cfg.OpenAI.Model = "gpt-4.1"
	cfg.OpenAI.Timeout = 45
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"image": {
			APIKey:   "image-key",
			BaseURL:  "https://image.example.test/v1",
			Model:    "gpt-image-1",
			Timeout:  90,
			APIStyle: "nanobanana",
		},
	}

	fallback := buildListingKitClientFallback(cfg, listingKitImageClientName)
	if fallback == nil {
		t.Fatal("expected fallback config")
	}
	if fallback.APIKey != "" {
		t.Fatalf("expected APIKey stripped, got %q", fallback.APIKey)
	}
	if fallback.BaseURL != "" {
		t.Fatalf("expected BaseURL stripped, got %q", fallback.BaseURL)
	}
	if fallback.Model != "" {
		t.Fatalf("expected Model stripped, got %q", fallback.Model)
	}
	if fallback.Timeout != 90*time.Second {
		t.Fatalf("expected Timeout preserved, got %v", fallback.Timeout)
	}
	if fallback.MaxRetries != 3 {
		t.Fatalf("expected MaxRetries preserved, got %d", fallback.MaxRetries)
	}
	if fallback.RetryDelay != time.Second {
		t.Fatalf("expected RetryDelay preserved, got %v", fallback.RetryDelay)
	}
}

func TestResolveStrictListingKitClientRejectsMissingResolvedConfig(t *testing.T) {
	resolver := &stubListingKitClientResolver{}
	cache := make(map[string]*openaiclient.Client)
	var mu sync.Mutex

	_, err := resolveStrictListingKitClient(context.Background(), "default", resolver, &openaiclient.ClientConfig{}, &mu, cache)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveStrictListingKitClientCachesResolvedClient(t *testing.T) {
	resolver := &stubListingKitClientResolver{
		resolved: &openaiclient.ResolvedClientConfig{
			CacheKey: "tenant-a:default",
			Config: &openaiclient.ClientConfig{
				APIKey:     "tenant-key",
				BaseURL:    "https://api.example.test/v1",
				Model:      "gpt-4.1-mini",
				Timeout:    30 * time.Second,
				MaxRetries: 2,
				RetryDelay: time.Second,
			},
		},
	}
	cache := make(map[string]*openaiclient.Client)
	var mu sync.Mutex

	first, err := resolveStrictListingKitClient(context.Background(), "default", resolver, &openaiclient.ClientConfig{Timeout: 10 * time.Second}, &mu, cache)
	if err != nil {
		t.Fatalf("first resolve returned error: %v", err)
	}
	second, err := resolveStrictListingKitClient(context.Background(), "default", resolver, &openaiclient.ClientConfig{Timeout: 10 * time.Second}, &mu, cache)
	if err != nil {
		t.Fatalf("second resolve returned error: %v", err)
	}
	if first != second {
		t.Fatal("expected cached client instance")
	}
	if resolver.lastFallback == nil {
		t.Fatal("expected fallback to be passed into resolver")
	}
	if len(cache) != 1 {
		t.Fatalf("expected one cached client, got %d", len(cache))
	}
}

func TestListingKitRoutedImageClientRoutesNanobananaWithConfiguredModel(t *testing.T) {
	nano := &stubListingKitImageGenerator{}
	gpt := &stubListingKitImageGenerator{}
	router := &listingKitRoutedImageClient{
		defaultModel: listingKitImageModelSelectorNano,
		defaultImage: nano,
		gptImage2:    gpt,
		nanobanana:   nano,
	}

	if _, err := router.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Model:  "nano-banana-fast",
		Prompt: "test",
	}); err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if nano.lastGenerate == nil {
		t.Fatal("expected nanobanana client to receive request")
	}
	if nano.lastGenerate.Model != "" {
		t.Fatalf("expected routed request model cleared, got %q", nano.lastGenerate.Model)
	}
}

func TestListingKitRoutedImageClientRoutesGPTImage2WithConfiguredModel(t *testing.T) {
	nano := &stubListingKitImageGenerator{}
	gpt := &stubListingKitImageGenerator{}
	router := &listingKitRoutedImageClient{
		defaultModel: listingKitImageModelSelectorNano,
		defaultImage: nano,
		gptImage2:    gpt,
		nanobanana:   nano,
	}

	if _, err := router.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Model:  listingKitImageModelSelectorGPTImage2,
		Prompt: "test",
	}); err != nil {
		t.Fatalf("EditImage returned error: %v", err)
	}
	if gpt.lastEdit == nil {
		t.Fatal("expected gpt image client to receive request")
	}
	if gpt.lastEdit.Model != "" {
		t.Fatalf("expected routed request model cleared, got %q", gpt.lastEdit.Model)
	}
}
