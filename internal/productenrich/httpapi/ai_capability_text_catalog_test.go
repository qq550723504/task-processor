package httpapi

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestProductEnrichTextCatalogResolvesFastCredential(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "fast-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "text-model", APIStyle: "openai"},
	}}
	decision, err := BuildProductEnrichTextCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityProductEnrichText,
		Operation: aicapability.OperationProductEnrichTextExtract, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if decision.ModelID != "text-model" || decision.CredentialReference != productEnrichTextClientName || decision.ConfigurationVersion != "fast-config-v1" {
		t.Fatalf("decision = %+v", decision)
	}
	if resolver.requestedClientName != productEnrichTextClientName {
		t.Fatalf("resolver client = %q, want %q", resolver.requestedClientName, productEnrichTextClientName)
	}
}

func TestProductEnrichTextCatalogDeniesTenantBeforeCredentialLookup(t *testing.T) {
	resolver := &productEnrichTextResolver{}
	_, err := BuildProductEnrichTextCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-b", UserID: "user-b", Capability: aicapability.CapabilityProductEnrichText,
		Operation: aicapability.OperationProductEnrichTextExtract,
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
	if resolver.requestedClientName != "" {
		t.Fatalf("resolver called for denied tenant: %q", resolver.requestedClientName)
	}
}

func TestProductEnrichTextCatalogPropagatesCredentialFailure(t *testing.T) {
	_, err := BuildProductEnrichTextCapabilityRouter(&productEnrichTextResolver{err: errors.New("credential lookup failed")}, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityProductEnrichText,
		Operation: aicapability.OperationProductEnrichTextExtract,
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("category = %q, want credential_unavailable", aicapability.CategoryOf(err))
	}
}

func TestProductEnrichVisionCatalogResolvesVisionCredential(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "vision-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "vision-model", APIStyle: "openai"},
	}}
	decision, err := BuildProductEnrichVisionCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityProductEnrichVision,
		Operation: aicapability.OperationProductEnrichImageAnalyze, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureVisionAnalyze},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if decision.ModelID != "vision-model" || decision.CredentialReference != productEnrichVisionClientName || decision.ConfigurationVersion != "vision-config-v1" {
		t.Fatalf("decision = %+v", decision)
	}
	if resolver.requestedClientName != productEnrichVisionClientName {
		t.Fatalf("resolver client = %q, want %q", resolver.requestedClientName, productEnrichVisionClientName)
	}
}

func TestProductEnrichVisionCatalogDeniesTenantBeforeCredentialLookup(t *testing.T) {
	resolver := &productEnrichTextResolver{}
	_, err := BuildProductEnrichVisionCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-b", UserID: "user-b", Capability: aicapability.CapabilityProductEnrichVision,
		Operation: aicapability.OperationProductEnrichImageAnalyze,
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
	if resolver.requestedClientName != "" {
		t.Fatalf("resolver called for denied tenant: %q", resolver.requestedClientName)
	}
}

func TestProductEnrichListingCatalogResolvesDefaultCredential(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "default-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "listing-model", APIStyle: "openai"},
	}}
	decision, err := BuildProductEnrichListingCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityProductEnrichListing,
		Operation: aicapability.OperationProductEnrichJSONGenerate, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if decision.ModelID != "listing-model" || decision.CredentialReference != productEnrichListingClientName || decision.ConfigurationVersion != "default-config-v1" {
		t.Fatalf("decision = %+v", decision)
	}
	if resolver.requestedClientName != productEnrichListingClientName {
		t.Fatalf("resolver client = %q, want %q", resolver.requestedClientName, productEnrichListingClientName)
	}
}

func TestProductEnrichListingCatalogDeniesTenantBeforeCredentialLookup(t *testing.T) {
	resolver := &productEnrichTextResolver{}
	_, err := BuildProductEnrichListingCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-b", UserID: "user-b", Capability: aicapability.CapabilityProductEnrichListing,
		Operation: aicapability.OperationProductEnrichJSONGenerate,
	})
	if aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
	if resolver.requestedClientName != "" {
		t.Fatalf("resolver called for denied tenant: %q", resolver.requestedClientName)
	}
}

func TestProductEnrichFusionCatalogResolvesDefaultCredential(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "fusion-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "fusion-model", APIStyle: "openai"},
	}}
	decision, err := BuildProductEnrichFusionCapabilityRouter(resolver, []string{"tenant-a"}).Decide(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityProductEnrichFusion,
		Operation: aicapability.OperationProductEnrichMultimodalFuse, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if decision.ModelID != "fusion-model" || decision.CredentialReference != productEnrichListingClientName || decision.ConfigurationVersion != "fusion-config-v1" {
		t.Fatalf("decision = %+v", decision)
	}
}

type productEnrichTextResolver struct {
	resolved            *openaiclient.ResolvedClientConfig
	err                 error
	requestedClientName string
}

func (r *productEnrichTextResolver) ResolveClientConfig(_ context.Context, clientName string, _ *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.requestedClientName = clientName
	return r.resolved, r.err
}
