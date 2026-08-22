package openai

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestManagerEffectiveConfigurationStaticVersionIsDeterministicNonSecretAndSensitive(t *testing.T) {
	ctx := context.Background()
	resolve := func(t *testing.T, name, key, model, baseURL string) EffectiveClientRoute {
		t.Helper()
		manager, err := NewManager(&ManagerConfig{Clients: map[string]*ClientConfig{
			name: testClientConfig(key, model, baseURL),
		}, DefaultClient: name})
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		t.Cleanup(func() { _ = manager.Close() })
		route, err := manager.ResolveEffectiveClientRoute(ctx, name)
		if err != nil {
			t.Fatalf("ResolveEffectiveClientRoute: %v", err)
		}
		return route
	}

	first := resolve(t, "default", "secret-one", "model-a", "https://one.test/v1")
	again := resolve(t, "default", "secret-one", "model-a", "https://one.test/v1")
	keyOnlyRotation := resolve(t, "default", "secret-two", "model-a", "https://one.test/v1")
	modelChange := resolve(t, "default", "secret-one", "model-b", "https://one.test/v1")
	baseURLChange := resolve(t, "default", "secret-one", "model-a", "https://two.test/v1")
	credentialIdentityChange := resolve(t, "fast", "secret-one", "model-a", "https://one.test/v1")

	if first.ConfigurationVersion == "" || first.ConfigurationVersion != again.ConfigurationVersion {
		t.Fatalf("static versions = %q/%q, want stable nonblank", first.ConfigurationVersion, again.ConfigurationVersion)
	}
	if strings.Contains(first.ConfigurationVersion, "secret-one") || strings.Contains(first.ConfigurationVersion, "secret-two") {
		t.Fatalf("configuration version leaked secret: %q", first.ConfigurationVersion)
	}
	if first.ConfigurationVersion != keyOnlyRotation.ConfigurationVersion {
		t.Fatalf("secret value must not enter version: %q/%q", first.ConfigurationVersion, keyOnlyRotation.ConfigurationVersion)
	}
	for label, changed := range map[string]EffectiveClientRoute{
		"model": modelChange, "base URL": baseURLChange, "credential identity": credentialIdentityChange,
	} {
		if changed.ConfigurationVersion == first.ConfigurationVersion {
			t.Fatalf("%s change did not change configuration version %q", label, first.ConfigurationVersion)
		}
	}
	if first.ProviderID != "openai" || first.ModelID != "model-a" || first.CredentialReference != "default" {
		t.Fatalf("effective route = %+v", first)
	}
}

func TestManagerEffectiveConfigurationUsesDBOverrideAndBindsExactVersion(t *testing.T) {
	var mu sync.Mutex
	var requests []capturedOpenAIRequest
	overrideServer := newCaptureChatServer(t, &requests, &mu)
	defer overrideServer.Close()
	resolver := &mutableEffectiveResolver{resolved: &ResolvedClientConfig{
		CacheKey: "db:v1", Config: testClientConfig("db-secret", "db-model", overrideServer.URL),
	}}
	manager, err := NewManager(&ManagerConfig{
		Clients:        map[string]*ClientConfig{"default": testClientConfig("static-secret", "static-model", "https://static.test/v1")},
		ConfigResolver: resolver, DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	ctx := WithIdentity(context.Background(), Identity{TenantID: "tenant-a", UserID: "user-a"})
	route, err := manager.ResolveEffectiveClientRoute(ctx, "default")
	if err != nil {
		t.Fatalf("ResolveEffectiveClientRoute: %v", err)
	}
	if route.ModelID != "db-model" || route.ConfigurationVersion == "" || strings.Contains(route.ConfigurationVersion, "db-secret") {
		t.Fatalf("DB effective route = %+v", route)
	}
	client, err := manager.GetClientWithRoute(ctx, "default", ImageRouteSelection{
		CredentialReference: route.CredentialReference, ConfigurationVersion: route.ConfigurationVersion,
	})
	if err != nil {
		t.Fatalf("GetClientWithRoute: %v", err)
	}
	callChat(t, client, ctx)
	if len(requests) != 1 || requests[0].Auth != "Bearer db-secret" || requests[0].Model != "db-model" {
		t.Fatalf("DB override requests = %#v", requests)
	}
}

func TestManagerBoundClientFailsClosedWhenEffectiveVersionRotates(t *testing.T) {
	var mu sync.Mutex
	var requests []capturedOpenAIRequest
	server := newCaptureChatServer(t, &requests, &mu)
	defer server.Close()
	resolver := &mutableEffectiveResolver{resolved: &ResolvedClientConfig{
		CacheKey: "db:v1", Config: testClientConfig("secret-v1", "model-v1", server.URL),
	}}
	manager, err := NewManager(&ManagerConfig{
		Clients:        map[string]*ClientConfig{"default": testClientConfig("static-secret", "static-model", server.URL)},
		ConfigResolver: resolver, DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	ctx := WithTenantID(context.Background(), "tenant-a")
	v1, err := manager.ResolveEffectiveClientRoute(ctx, "default")
	if err != nil {
		t.Fatalf("resolve v1: %v", err)
	}
	resolver.set(&ResolvedClientConfig{CacheKey: "db:v2", Config: testClientConfig("secret-v2", "model-v2", server.URL)})
	_, err = manager.GetClientWithRoute(ctx, "default", ImageRouteSelection{
		CredentialReference: v1.CredentialReference, ConfigurationVersion: v1.ConfigurationVersion,
	})
	if !errors.Is(err, ErrClientConfigurationChanged) {
		t.Fatalf("bound rotation error = %v, want ErrClientConfigurationChanged", err)
	}
	if len(requests) != 0 {
		t.Fatalf("provider requests after version mismatch = %d, want 0", len(requests))
	}
}

func TestManagerEffectiveConfigurationReportsGenuinelyUnavailableCandidate(t *testing.T) {
	manager, err := NewManager(&ManagerConfig{
		Clients:        map[string]*ClientConfig{"default": testClientConfig("secret", "model", "https://example.test/v1")},
		ConfigResolver: &mutableEffectiveResolver{}, DefaultClient: "default",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	_, err = manager.ResolveEffectiveClientRoute(WithTenantID(context.Background(), "tenant-a"), "fast")
	if !errors.Is(err, ErrClientConfigurationUnavailable) {
		t.Fatalf("missing candidate error = %v, want ErrClientConfigurationUnavailable", err)
	}
}

type mutableEffectiveResolver struct {
	mu       sync.RWMutex
	resolved *ResolvedClientConfig
}

func (r *mutableEffectiveResolver) ResolveClientConfig(context.Context, string, *ClientConfig) (*ResolvedClientConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.resolved == nil {
		return nil, nil
	}
	return &ResolvedClientConfig{CacheKey: r.resolved.CacheKey, Config: cloneClientConfig(r.resolved.Config)}, nil
}

func (r *mutableEffectiveResolver) set(resolved *ResolvedClientConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolved = resolved
}
