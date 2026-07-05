package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

type stubListingKitClientResolver struct {
	resolved     *openaiclient.ResolvedClientConfig
	err          error
	lastName     string
	lastFallback *openaiclient.ClientConfig
}

type stubListingKitImageGenerator struct {
	lastGenerate   *openaiclient.ImageGenerateRequest
	lastEdit       *openaiclient.ImageEditRequest
	asyncSupported bool
	submitResponse *openaiclient.ImageAsyncSubmitResponse
	queryResponse  *openaiclient.ImageAsyncQueryResponse
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

func (s *stubListingKitImageGenerator) SupportsAsyncImageGeneration() bool {
	return s.asyncSupported
}

func (s *stubListingKitImageGenerator) SubmitImageGeneration(_ context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	s.lastGenerate = req
	return s.submitResponse, nil
}

func (s *stubListingKitImageGenerator) SubmitImageEdit(_ context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	s.lastEdit = req
	return s.submitResponse, nil
}

func (s *stubListingKitImageGenerator) QueryImageGeneration(_ context.Context, _ string) (*openaiclient.ImageAsyncQueryResponse, error) {
	return s.queryResponse, nil
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

func TestListingKitRoutedImageClientUsesGPTImage2ByDefault(t *testing.T) {
	nano := &stubListingKitImageGenerator{}
	gpt := &stubListingKitImageGenerator{}
	router := &listingKitRoutedImageClient{
		defaultModel: listingKitImageModelSelectorGPTImage2,
		defaultImage: gpt,
		gptImage2:    gpt,
		nanobanana:   nano,
	}

	if _, err := router.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "test",
	}); err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if gpt.lastGenerate == nil {
		t.Fatal("expected gpt image client to receive default request")
	}
	if gpt.lastGenerate.Model != "" {
		t.Fatalf("expected routed request model cleared, got %q", gpt.lastGenerate.Model)
	}
	if nano.lastGenerate != nil {
		t.Fatal("did not expect nanobanana client to receive default request")
	}
}

func TestAdaptListingKitAIImageGeneratorDelegatesAsyncCapability(t *testing.T) {
	acceptedAt := time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC)
	generator := &stubListingKitImageGenerator{
		asyncSupported: true,
		submitResponse: &openaiclient.ImageAsyncSubmitResponse{
			JobID:             "job-1",
			RequestID:         "req-1",
			Provider:          "grsai",
			RawSubmitResponse: `{"id":"job-1"}`,
			AcceptedAt:        acceptedAt,
		},
		queryResponse: &openaiclient.ImageAsyncQueryResponse{
			JobID:             "job-1",
			RequestID:         "req-1",
			Provider:          "grsai",
			Status:            "running",
			RawResultResponse: `{"id":"job-1","status":"running"}`,
		},
	}

	adapted := adaptListingKitAIImageGenerator(generator)
	asyncGenerator, ok := adapted.(listingkit.AIAsyncImageGenerator)
	if !ok {
		t.Fatal("adapted generator does not implement AIAsyncImageGenerator")
	}
	if !asyncGenerator.SupportsAsyncImageGeneration() {
		t.Fatal("SupportsAsyncImageGeneration() = false, want true")
	}

	submit, err := asyncGenerator.SubmitImageGeneration(context.Background(), &listingkit.AIImageGenerateRequest{
		Model:  "nano-banana-fast",
		Prompt: "flat artwork",
	})
	if err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if submit == nil || submit.JobID != "job-1" {
		t.Fatalf("submit = %+v, want job-1 metadata", submit)
	}
	if submit.AcceptedAt != acceptedAt {
		t.Fatalf("accepted at = %v, want %v", submit.AcceptedAt, acceptedAt)
	}

	query, err := asyncGenerator.QueryImageGeneration(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if query == nil || query.Status != listingkit.AIImageAsyncResultRunning {
		t.Fatalf("query = %+v, want running result", query)
	}
	if query.Provider != "grsai" {
		t.Fatalf("query provider = %q, want grsai", query.Provider)
	}

	editSubmit, err := asyncGenerator.SubmitImageEdit(context.Background(), &listingkit.AIImageEditRequest{
		Model:     "gpt-image-2",
		Prompt:    "flat artwork",
		ImageURL:  "https://example.com/a.png",
		ImageURLs: []string{"https://example.com/a.png", "https://example.com/b.png"},
	})
	if err != nil {
		t.Fatalf("SubmitImageEdit() error = %v", err)
	}
	if editSubmit == nil || generator.lastEdit == nil {
		t.Fatal("expected async edit submission to delegate to provider")
	}
}

func TestBuildStrictListingKitImageClientUsesGRSAIGenerateProtocolForGPTImage2(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req["model"] != "gpt-image-2" {
				t.Fatalf("model = %#v", req["model"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "job-1",
				"status":   "succeeded",
				"results":  []map[string]any{{"url": serverURL + "/generated.png"}},
				"progress": 100,
			})
		case "/generated.png":
			_, _ = w.Write([]byte("png"))
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	resolver := &stubListingKitClientResolver{
		resolved: &openaiclient.ResolvedClientConfig{
			CacheKey: "tenant-a:image_gpt_image_2",
			Config: &openaiclient.ClientConfig{
				APIKey:     "tenant-key",
				BaseURL:    server.URL + "/grsai/v1",
				Model:      "gpt-image-2",
				Timeout:    time.Second,
				MaxRetries: 1,
				RetryDelay: time.Millisecond,
			},
		},
	}

	client := buildStrictListingKitImageClient(&config.Config{}, resolver, listingKitImageClientNameGPTImage2)
	if client == nil {
		t.Fatal("expected image client")
	}
	if _, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "test",
		Size:   "1024x1024",
	}); err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
}

func TestBuildStrictListingKitImageClientReportsAsyncSupportFromResolvedClient(t *testing.T) {
	resolver := &stubListingKitClientResolver{
		resolved: &openaiclient.ResolvedClientConfig{
			CacheKey: "tenant-a:image_gpt_image_2",
			Config: &openaiclient.ClientConfig{
				APIKey:     "tenant-key",
				BaseURL:    "https://example.test/v1",
				Model:      "gpt-image-2",
				Timeout:    time.Second,
				MaxRetries: 1,
				RetryDelay: time.Millisecond,
			},
		},
	}
	client := &strictListingKitConfiguredImageClient{
		clientName: listingKitImageClientNameGPTImage2,
		resolver:   resolver,
		cache:      make(map[string]openaiclient.ImageGenerator),
		build: func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error) {
			return &stubListingKitImageGenerator{asyncSupported: true}, nil
		},
	}

	if !client.SupportsAsyncImageGeneration() {
		t.Fatal("expected strict image client to report async support from resolved client")
	}
}

func TestBuildStrictListingKitNanobananaImageClientUsesGeminiProtocolWhenConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-2.5-flash-image:generateContent" {
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"inlineData": map[string]any{
									"mimeType": "image/png",
									"data":     "cG5n",
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	resolver := &stubListingKitClientResolver{
		resolved: &openaiclient.ResolvedClientConfig{
			CacheKey: "tenant-a:image_nanobanana",
			Config: &openaiclient.ClientConfig{
				APIKey:     "tenant-key",
				BaseURL:    server.URL + "/v1beta",
				Model:      "gemini-2.5-flash-image",
				APIStyle:   "gemini",
				Timeout:    time.Second,
				MaxRetries: 1,
				RetryDelay: time.Millisecond,
			},
		},
	}

	client := buildStrictListingKitNanobananaImageClient(&config.Config{}, resolver, listingKitImageClientNameNanobanana)
	if client == nil {
		t.Fatal("expected image client")
	}
	if _, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "test",
		Size:   "1024x1024",
	}); err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
}

func TestEnforceListingKitImageClientTimeoutClampsImageClients(t *testing.T) {
	cfg := &openaiclient.ClientConfig{Timeout: 60 * time.Second}

	got := enforceListingKitImageClientTimeout(listingKitImageClientNameNanobanana, cfg)
	if got == cfg {
		t.Fatal("expected timeout clamp to clone image client config")
	}
	if got.Timeout != listingKitStudioImageMinTimeout {
		t.Fatalf("timeout = %v, want %v", got.Timeout, listingKitStudioImageMinTimeout)
	}
	if cfg.Timeout != 60*time.Second {
		t.Fatalf("original timeout mutated = %v, want 60s", cfg.Timeout)
	}
}

func TestEnforceListingKitImageClientTimeoutLeavesNonImageClientsUnchanged(t *testing.T) {
	cfg := &openaiclient.ClientConfig{Timeout: 60 * time.Second}

	got := enforceListingKitImageClientTimeout("default", cfg)
	if got != cfg {
		t.Fatal("expected non-image client config to be reused")
	}
}
