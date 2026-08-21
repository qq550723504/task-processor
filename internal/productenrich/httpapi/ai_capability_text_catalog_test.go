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

type productEnrichTextResolver struct {
	resolved            *openaiclient.ResolvedClientConfig
	err                 error
	requestedClientName string
}

func (r *productEnrichTextResolver) ResolveClientConfig(_ context.Context, clientName string, _ *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.requestedClientName = clientName
	return r.resolved, r.err
}
