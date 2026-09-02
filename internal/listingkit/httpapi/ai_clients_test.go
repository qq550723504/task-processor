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
	err          error
	lastName     string
	lastFallback *openaiclient.ClientConfig
	lastIdentity openaiclient.Identity
}

func (r *stubListingKitClientResolver) ResolveClientConfig(ctx context.Context, clientName string, fallback *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.lastName = clientName
	r.lastFallback = fallback
	r.lastIdentity = openaiclient.IdentityFromContext(ctx)
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
		"default": {
			APIKey:   "tenant-key",
			BaseURL:  "https://chat.example.test/v1",
			Model:    "gpt-4.1-mini",
			Timeout:  90,
			APIStyle: "openai",
		},
	}

	fallback := buildListingKitClientFallback(cfg, "default")
	if fallback == nil {
		t.Fatal("expected fallback config")
	}
	if fallback.APIKey != "" || fallback.BaseURL != "" || fallback.Model != "" {
		t.Fatalf("sensitive fallback fields were not stripped: %#v", fallback)
	}
	if fallback.Timeout != 90*time.Second {
		t.Fatalf("expected Timeout preserved, got %v", fallback.Timeout)
	}
	if fallback.MaxRetries != 3 || fallback.RetryDelay != time.Second {
		t.Fatalf("retry policy was not preserved: %#v", fallback)
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
