package httpapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
)

type stubListingKitClientResolver struct {
	resolved     *openaiclient.ResolvedClientConfig
	lastFallback *openaiclient.ClientConfig
}

func (r *stubListingKitClientResolver) ResolveClientConfig(_ context.Context, _ string, fallback *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.lastFallback = fallback
	return r.resolved, nil
}

func TestBuildListingKitClientFallbackStripsSensitiveFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenAI.APIKey = "shared-key"
	cfg.OpenAI.BaseURL = "https://default.example.test/v1"
	cfg.OpenAI.Model = "gpt-4.1"
	cfg.OpenAI.Timeout = 45
	cfg.OpenAI.Clients = map[string]config.OpenAIClientConfig{
		"default": {APIKey: "tenant-key", BaseURL: "https://chat.example.test/v1", Model: "gpt-4.1-mini", Timeout: 90, APIStyle: "openai"},
	}

	fallback := buildListingKitClientFallback(cfg, "default")
	if fallback == nil {
		t.Fatal("expected fallback config")
	}
	if fallback.APIKey != "" || fallback.BaseURL != "" || fallback.Model != "" {
		t.Fatalf("sensitive fallback fields were not stripped: %#v", fallback)
	}
	if fallback.Timeout != 90*time.Second || fallback.MaxRetries != 3 || fallback.RetryDelay != time.Second {
		t.Fatalf("non-secret fallback policy was not preserved: %#v", fallback)
	}
}

func TestResolveStrictListingKitClientRejectsMissingResolvedConfig(t *testing.T) {
	resolver := &stubListingKitClientResolver{}
	_, err := resolveStrictListingKitClient(context.Background(), "default", resolver, &openaiclient.ClientConfig{}, &sync.Mutex{}, make(map[string]*openaiclient.Client))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveStrictListingKitClientCachesResolvedClient(t *testing.T) {
	resolver := &stubListingKitClientResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "tenant-a:default",
		Config:   &openaiclient.ClientConfig{APIKey: "tenant-key", BaseURL: "https://api.example.test/v1", Model: "gpt-4.1-mini", Timeout: 30 * time.Second, MaxRetries: 2, RetryDelay: time.Second},
	}}
	cache := make(map[string]*openaiclient.Client)
	mu := &sync.Mutex{}
	first, err := resolveStrictListingKitClient(context.Background(), "default", resolver, &openaiclient.ClientConfig{Timeout: 10 * time.Second}, mu, cache)
	if err != nil {
		t.Fatalf("first resolve returned error: %v", err)
	}
	second, err := resolveStrictListingKitClient(context.Background(), "default", resolver, &openaiclient.ClientConfig{Timeout: 10 * time.Second}, mu, cache)
	if err != nil {
		t.Fatalf("second resolve returned error: %v", err)
	}
	if first != second || len(cache) != 1 || resolver.lastFallback == nil {
		t.Fatalf("cache/fallback mismatch: first=%p second=%p cache=%d fallback=%#v", first, second, len(cache), resolver.lastFallback)
	}
}
