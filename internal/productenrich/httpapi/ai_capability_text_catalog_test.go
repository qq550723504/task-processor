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
	decision, err := BuildProductEnrichTextCapabilityRouter(resolver).Decide(context.Background(), aicapability.RouteRequest{
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

func TestProductEnrichCatalogsAcceptGeminiCredentials(t *testing.T) {
	tests := []struct {
		name     string
		resolver func(*productEnrichTextResolver) aicapability.Router
		cap      aicapability.Capability
		op       aicapability.Operation
		feature  aicapability.ModelFeature
	}{
		{"text", func(r *productEnrichTextResolver) aicapability.Router {
			return BuildProductEnrichTextCapabilityRouter(r)
		}, aicapability.CapabilityProductEnrichText, aicapability.OperationProductEnrichTextExtract, aicapability.FeatureTextGenerate},
		{"vision", func(r *productEnrichTextResolver) aicapability.Router {
			return BuildProductEnrichVisionCapabilityRouter(r)
		}, aicapability.CapabilityProductEnrichVision, aicapability.OperationProductEnrichImageAnalyze, aicapability.FeatureVisionAnalyze},
		{"listing", func(r *productEnrichTextResolver) aicapability.Router {
			return BuildProductEnrichListingCapabilityRouter(r)
		}, aicapability.CapabilityProductEnrichListing, aicapability.OperationProductEnrichJSONGenerate, aicapability.FeatureTextGenerate},
		{"fusion", func(r *productEnrichTextResolver) aicapability.Router {
			return BuildProductEnrichFusionCapabilityRouter(r)
		}, aicapability.CapabilityProductEnrichFusion, aicapability.OperationProductEnrichMultimodalFuse, aicapability.FeatureTextGenerate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
				CacheKey: "gemini-config-v1",
				Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "gemini-2.5-flash", APIStyle: "gemini"},
			}}
			decision, err := tt.resolver(resolver).Decide(context.Background(), aicapability.RouteRequest{
				TenantID: "tenant-a", UserID: "user-a", Capability: tt.cap, Operation: tt.op,
				RequiredFeatures: []aicapability.ModelFeature{tt.feature},
			})
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if decision.ProviderID != "gemini" {
				t.Fatalf("provider = %q, want gemini", decision.ProviderID)
			}
		})
	}
}

func TestProductEnrichExecutionPlannerReturnsLegacyForInactiveTenantWithoutRouting(t *testing.T) {
	router := &recordingProductEnrichRouter{}
	planner := BuildProductEnrichExecutionPlanner(router, []string{"tenant-a"}, []string{"fast", "default"})

	plan, err := planner.Plan(context.Background(), aicapability.RouteRequest{
		TenantID: " tenant-b ", UserID: "user-b", Capability: aicapability.CapabilityProductEnrichText,
		Operation: aicapability.OperationProductEnrichTextExtract,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if router.called {
		t.Fatal("router called for inactive tenant")
	}
	if plan.Mode != aicapability.RoutingModeLegacy || plan.RouteOutcome != aicapability.RouteOutcomeLegacy {
		t.Fatalf("plan = %+v", plan)
	}
	if len(plan.LegacyClients) != 2 || plan.LegacyClients[0] != "fast" || plan.LegacyClients[1] != "default" {
		t.Fatalf("legacy clients = %#v", plan.LegacyClients)
	}
}

func TestProductEnrichExecutionPlannerDelegatesActiveTenant(t *testing.T) {
	router := &recordingProductEnrichRouter{decision: aicapability.RouteDecision{
		Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextExtract,
		ProviderID: "openai", ModelID: "text-model", RoutingKey: "productenrich-text", CredentialReference: "fast",
	}}
	planner := BuildProductEnrichExecutionPlanner(router, []string{" tenant-a "}, []string{"fast", "default"})

	plan, err := planner.Plan(context.Background(), aicapability.RouteRequest{
		TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityProductEnrichText,
		Operation: aicapability.OperationProductEnrichTextExtract,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !router.called || plan.Mode != aicapability.RoutingModeActive || plan.RouteOutcome != aicapability.RouteOutcomeActive {
		t.Fatalf("router called/plan = %v/%+v", router.called, plan)
	}
}

func TestProductEnrichTextCatalogPropagatesCredentialFailure(t *testing.T) {
	_, err := BuildProductEnrichTextCapabilityRouter(&productEnrichTextResolver{err: errors.New("credential lookup failed")}).Decide(context.Background(), aicapability.RouteRequest{
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
	decision, err := BuildProductEnrichVisionCapabilityRouter(resolver).Decide(context.Background(), aicapability.RouteRequest{
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

func TestProductEnrichListingCatalogResolvesDefaultCredential(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "default-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "listing-model", APIStyle: "openai"},
	}}
	decision, err := BuildProductEnrichListingCapabilityRouter(resolver).Decide(context.Background(), aicapability.RouteRequest{
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

func TestProductEnrichFusionCatalogResolvesDefaultCredential(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "fusion-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "fusion-model", APIStyle: "openai"},
	}}
	decision, err := BuildProductEnrichFusionCapabilityRouter(resolver).Decide(context.Background(), aicapability.RouteRequest{
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

func TestProductEnrichActiveRoutersRejectInvalidOperationsBeforeCredentialLookup(t *testing.T) {
	tests := []struct {
		name  string
		build func(openaiclient.ClientConfigResolver) aicapability.Router
		cap   aicapability.Capability
		badOp aicapability.Operation
	}{
		{"text", BuildProductEnrichTextCapabilityRouter, aicapability.CapabilityProductEnrichText, aicapability.OperationProductEnrichImageAnalyze},
		{"vision", BuildProductEnrichVisionCapabilityRouter, aicapability.CapabilityProductEnrichVision, aicapability.OperationProductEnrichTextExtract},
		{"listing", BuildProductEnrichListingCapabilityRouter, aicapability.CapabilityProductEnrichListing, aicapability.OperationProductEnrichImageAnalyze},
		{"fusion", BuildProductEnrichFusionCapabilityRouter, aicapability.CapabilityProductEnrichFusion, aicapability.OperationProductEnrichTextExtract},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &productEnrichTextResolver{}
			_, err := tt.build(resolver).Decide(context.Background(), aicapability.RouteRequest{
				TenantID: "tenant-a", UserID: "user-a", Capability: tt.cap, Operation: tt.badOp,
			})
			if aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
				t.Fatalf("category = %q, want policy_denied", aicapability.CategoryOf(err))
			}
			if resolver.requestedClientName != "" {
				t.Fatalf("resolver called for invalid operation: %q", resolver.requestedClientName)
			}
		})
	}
}

func TestProductEnrichLegacyRouteMetadataResolverUsesTenantAwareClientConfig(t *testing.T) {
	resolver := &productEnrichTextResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "fast-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test/v1", Model: "legacy-model", APIStyle: "gemini"},
	}}
	metadata := BuildProductEnrichLegacyRouteMetadataResolver(resolver)

	decision, err := metadata.ResolveLegacyRoute(context.Background(), " fast ")
	if err != nil {
		t.Fatalf("ResolveLegacyRoute: %v", err)
	}
	if decision.ProviderID != "gemini" || decision.ModelID != "legacy-model" || decision.RoutingKey != "fast" || decision.CredentialReference != "fast" || decision.ConfigurationVersion != "fast-config-v1" {
		t.Fatalf("decision = %+v", decision)
	}
	if resolver.requestedClientName != "fast" {
		t.Fatalf("resolver client = %q, want fast", resolver.requestedClientName)
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

type recordingProductEnrichRouter struct {
	decision aicapability.RouteDecision
	err      error
	called   bool
}

func (r *recordingProductEnrichRouter) Decide(context.Context, aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	r.called = true
	return r.decision, r.err
}
