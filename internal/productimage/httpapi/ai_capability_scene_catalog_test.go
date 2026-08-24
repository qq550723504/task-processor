package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestProductImageSceneCatalogMapsGPTImageCredentialWithoutExposingSecret(t *testing.T) {
	resolver := &productImageSceneResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "image-config-v1",
		Config: &openaiclient.ClientConfig{
			APIKey:   "super-secret",
			BaseURL:  "https://image.example.test/v1",
			Model:    "gpt-image-2",
			APIStyle: "openai",
		},
	}}

	router := BuildProductImageSceneCapabilityRouter(resolver, []string{"tenant-a"})
	decision, err := router.Decide(context.Background(), aicapability.RouteRequest{
		TenantID:         "tenant-a",
		Capability:       aicapability.CapabilityProductImageScene,
		Operation:        aicapability.OperationProductImageSceneGenerate,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureImageEdit},
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision.ModelID != "gpt-image-2" || decision.ProviderID != "openai" {
		t.Fatalf("decision = %+v", decision)
	}
	if decision.CredentialReference != productImageSceneClientName || decision.ConfigurationVersion != "image-config-v1" {
		t.Fatalf("credential binding = %+v", decision)
	}
	if resolver.requestedClientName != productImageSceneClientName {
		t.Fatalf("resolver client name = %q, want %q", resolver.requestedClientName, productImageSceneClientName)
	}
	if strings.Contains(strings.Join([]string{decision.ProviderID, decision.ModelID, decision.RoutingKey, decision.CredentialReference}, " "), "super-secret") {
		t.Fatal("route decision exposed API key")
	}
}

func TestProductImageSceneCatalogRejectsUnavailableCredential(t *testing.T) {
	_, err := BuildProductImageSceneCapabilityRouter(&productImageSceneResolver{err: errors.New("credential lookup failed")}, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID:         "tenant-a",
		Capability:       aicapability.CapabilityProductImageScene,
		Operation:        aicapability.OperationProductImageSceneGenerate,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureImageEdit},
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error category = %q, want %q (err %v)", aicapability.CategoryOf(err), aicapability.ErrorCredentialUnavailable, err)
	}
}

func TestProductImageSceneCatalogDeniesTenantOutsideAllowlistBeforeCredentialLookup(t *testing.T) {
	resolver := &productImageSceneResolver{}
	router := BuildProductImageSceneCapabilityRouter(resolver, []string{"tenant-a"})
	_, err := router.Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-b", Capability: aicapability.CapabilityProductImageScene,
		Operation: aicapability.OperationProductImageSceneGenerate,
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
	if resolver.requestedClientName != "" {
		t.Fatalf("resolver was called for denied tenant with client %q", resolver.requestedClientName)
	}
}

func TestProductImageSceneCatalogDeniesEveryTenantWithEmptyAllowlist(t *testing.T) {
	router := BuildProductImageSceneCapabilityRouter(&productImageSceneResolver{}, nil)
	_, err := router.Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", Capability: aicapability.CapabilityProductImageScene,
		Operation: aicapability.OperationProductImageSceneGenerate,
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
}

func TestProductImageSceneCatalogRejectsUnsupportedAPIStyle(t *testing.T) {
	resolver := &productImageSceneResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "db:1",
		Config: &openaiclient.ClientConfig{
			APIKey:   "secret",
			BaseURL:  "https://example.test/v1",
			Model:    "image-model",
			APIStyle: "nanobanana",
		},
	}}
	_, err := (&productImageSceneModelCatalog{resolver: resolver}).ResolveModel(context.Background(), productImageSceneRoutingKey)
	if aicapability.CategoryOf(err) != aicapability.ErrorCapabilityUnavailable {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorCapabilityUnavailable)
	}
}

func TestProductImageSceneCatalogRejectsMissingConfigurationVersion(t *testing.T) {
	resolver := &productImageSceneResolver{resolved: &openaiclient.ResolvedClientConfig{
		Config: &openaiclient.ClientConfig{
			APIKey:   "secret",
			BaseURL:  "https://example.test/v1",
			Model:    "image-model",
			APIStyle: "openai",
		},
	}}
	_, err := (&productImageSceneModelCatalog{resolver: resolver}).ResolveModel(context.Background(), productImageSceneRoutingKey)
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorCredentialUnavailable)
	}
}

type productImageSceneResolver struct {
	resolved            *openaiclient.ResolvedClientConfig
	err                 error
	requestedClientName string
}

func (r *productImageSceneResolver) ResolveClientConfig(_ context.Context, clientName string, _ *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.requestedClientName = clientName
	return r.resolved, r.err
}
