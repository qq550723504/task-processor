package aicapability

import (
	"context"
	"errors"
	"testing"
)

type stubCatalog struct {
	models map[string]ModelDefinition
}

func (s stubCatalog) ResolveModel(_ context.Context, routingKey string) (ModelDefinition, error) {
	model, ok := s.models[routingKey]
	if !ok {
		return ModelDefinition{}, errors.New("model not found")
	}
	return model, nil
}

type staticPolicyResolver struct {
	policy TenantModelPolicy
	err    error
	calls  int
}

func (s *staticPolicyResolver) ResolvePolicy(_ context.Context, _ RouteRequest) (TenantModelPolicy, error) {
	s.calls++
	return s.policy, s.err
}

func TestPolicyRouterUsesRequestedRoutingKeyWhenAllowed(t *testing.T) {
	policy := TenantModelPolicy{
		TenantID:             "tenant-a",
		Capability:           CapabilityListingKitStudioImage,
		AllowedRoutingKeys:   []string{"gpt-image-2", "nanobanana"},
		PreferredRoutingKeys: []string{"gpt-image-2"},
		Version:              "legacy-studio-v1",
	}
	catalog := stubCatalog{models: map[string]ModelDefinition{
		"nanobanana": {
			ProviderID:           "grsai",
			ModelID:              "nano-banana-pro",
			RoutingKey:           "nanobanana",
			CredentialReference:  "image_nanobanana",
			Features:             []ModelFeature{FeatureImageGenerate, FeatureImageEdit, FeatureAsyncImageJob},
			Enabled:              true,
			ConfigurationVersion: "credential-7",
		},
	}}
	resolver := &staticPolicyResolver{policy: policy}
	router := NewPolicyRouter(catalog, resolver)

	decision, err := router.Decide(context.Background(), RouteRequest{
		TenantID:            " tenant-a ",
		Capability:          CapabilityListingKitStudioImage,
		Operation:           OperationImageEdit,
		RequestedRoutingKey: " nanobanana ",
		RequiredFeatures:    []ModelFeature{FeatureImageEdit},
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("ResolvePolicy calls = %d, want 1", resolver.calls)
	}
	if decision.ProviderID != "grsai" || decision.ModelID != "nano-banana-pro" || decision.RoutingKey != "nanobanana" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if decision.PolicyVersion != "legacy-studio-v1" || decision.ConfigurationVersion != "credential-7" || decision.FallbackIndex != 0 {
		t.Fatalf("unexpected decision metadata: %#v", decision)
	}
}

func TestPolicyRouterUsesPreferredKeyWhenRequestIsNotAllowed(t *testing.T) {
	router := NewPolicyRouter(stubCatalog{models: map[string]ModelDefinition{
		"gpt-image-2": {RoutingKey: "gpt-image-2", Enabled: true},
	}}, &staticPolicyResolver{policy: TenantModelPolicy{
		TenantID:             "tenant-a",
		Capability:           CapabilityListingKitStudioImage,
		AllowedRoutingKeys:   []string{"gpt-image-2"},
		PreferredRoutingKeys: []string{"gpt-image-2"},
	}})

	decision, err := router.Decide(context.Background(), RouteRequest{
		TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage, Operation: OperationImageGenerate, RequestedRoutingKey: "blocked",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision.RoutingKey != "gpt-image-2" {
		t.Fatalf("RoutingKey = %q, want gpt-image-2", decision.RoutingKey)
	}
}

func TestPolicyRouterOnlyFallsBackWhenPolicyAllowsIt(t *testing.T) {
	models := map[string]ModelDefinition{
		"first":  {RoutingKey: "first", Enabled: false},
		"second": {RoutingKey: "second", Enabled: true},
	}
	request := RouteRequest{TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage, Operation: OperationImageGenerate}

	for _, tc := range []struct {
		name          string
		allowFallback bool
		wantKey       string
		wantCategory  ErrorCategory
	}{
		{name: "disabled without fallback", wantCategory: ErrorCapabilityUnavailable},
		{name: "disabled with fallback", allowFallback: true, wantKey: "second"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewPolicyRouter(stubCatalog{models: models}, &staticPolicyResolver{policy: TenantModelPolicy{
				TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage, PreferredRoutingKeys: []string{"first", "second"}, AllowCrossProviderFallback: tc.allowFallback,
			}})
			decision, err := router.Decide(context.Background(), request)
			if tc.wantCategory != "" {
				if CategoryOf(err) != tc.wantCategory {
					t.Fatalf("CategoryOf(err) = %q, want %q (err=%v)", CategoryOf(err), tc.wantCategory, err)
				}
				return
			}
			if err != nil || decision.RoutingKey != tc.wantKey || decision.FallbackIndex != 1 {
				t.Fatalf("decision=%#v err=%v", decision, err)
			}
		})
	}
}

func TestPolicyRouterRejectsModelWithoutRequiredFeatureOrDataTag(t *testing.T) {
	router := NewPolicyRouter(stubCatalog{models: map[string]ModelDefinition{
		"gpt-image-2": {RoutingKey: "gpt-image-2", Features: []ModelFeature{FeatureImageGenerate}, DataPolicyTags: []string{"public"}, Enabled: true},
	}}, &staticPolicyResolver{policy: TenantModelPolicy{
		TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage, PreferredRoutingKeys: []string{"gpt-image-2"}, RequiredDataPolicyTags: []string{"restricted"},
	}})

	_, err := router.Decide(context.Background(), RouteRequest{
		TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage, Operation: OperationImageEdit, RequiredFeatures: []ModelFeature{FeatureImageEdit},
	})
	if CategoryOf(err) != ErrorCapabilityUnavailable {
		t.Fatalf("CategoryOf(err) = %q, want %q", CategoryOf(err), ErrorCapabilityUnavailable)
	}
}

func TestPolicyRouterRejectsPolicyResolverFailure(t *testing.T) {
	router := NewPolicyRouter(stubCatalog{}, &staticPolicyResolver{err: errors.New("policy missing")})
	_, err := router.Decide(context.Background(), RouteRequest{
		TenantID: "tenant-a", Capability: CapabilityListingKitStudioImage, Operation: OperationImageGenerate,
	})
	if CategoryOf(err) != ErrorPolicyDenied {
		t.Fatalf("CategoryOf(err) = %q, want %q", CategoryOf(err), ErrorPolicyDenied)
	}
}

func TestCategoryOf(t *testing.T) {
	if got := CategoryOf(nil); got != "" {
		t.Fatalf("CategoryOf(nil) = %q", got)
	}
	if got := CategoryOf(errors.New("plain")); got != ErrorUnknown {
		t.Fatalf("CategoryOf(plain) = %q, want %q", got, ErrorUnknown)
	}
	if got := CategoryOf(NewError(ErrorProviderTimeout, "image_generate", errors.New("timeout"))); got != ErrorProviderTimeout {
		t.Fatalf("CategoryOf(capability error) = %q", got)
	}
}
