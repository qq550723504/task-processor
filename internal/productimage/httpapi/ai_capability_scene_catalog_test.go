package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestProductImageSceneCatalogMapsImageCredentialWithoutExposingSecret(t *testing.T) {
	resolver := &productImageSceneResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "image-config-v1",
		Config: &openaiclient.ClientConfig{
			APIKey:   "super-secret",
			BaseURL:  "https://image.example.test/v1",
			Model:    "gpt-image-2",
			APIStyle: "openai",
		},
	}}

	router := BuildProductImageSceneCapabilityRouter(resolver)
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
	if decision.CredentialReference != "image" || decision.ConfigurationVersion != "image-config-v1" {
		t.Fatalf("credential binding = %+v", decision)
	}
	if strings.Contains(strings.Join([]string{decision.ProviderID, decision.ModelID, decision.RoutingKey, decision.CredentialReference}, " "), "super-secret") {
		t.Fatal("route decision exposed API key")
	}
}

func TestProductImageSceneCatalogRejectsUnavailableCredential(t *testing.T) {
	_, err := BuildProductImageSceneCapabilityRouter(&productImageSceneResolver{err: errors.New("credential lookup failed")}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID:         "tenant-a",
		Capability:       aicapability.CapabilityProductImageScene,
		Operation:        aicapability.OperationProductImageSceneGenerate,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureImageEdit},
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error category = %q, want %q (err %v)", aicapability.CategoryOf(err), aicapability.ErrorCredentialUnavailable, err)
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
	resolved *openaiclient.ResolvedClientConfig
	err      error
}

func (r *productImageSceneResolver) ResolveClientConfig(context.Context, string, *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	return r.resolved, r.err
}
