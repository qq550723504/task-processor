package httpapi

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestStudioCapabilityCatalogMapsSelectorsWithoutExposingCredentials(t *testing.T) {
	tests := []struct {
		name              string
		routingKey        string
		config            *openaiclient.ClientConfig
		wantClientName    string
		wantModel         string
		wantProvider      string
		wantAsync         bool
		wantConfiguration string
	}{
		{
			name:              "default selector uses configured GPT image model",
			config:            &openaiclient.ClientConfig{APIKey: "secret-default", BaseURL: "https://openai.example.test/v1", Model: "configured-gpt"},
			wantClientName:    listingKitImageClientNameGPTImage2,
			wantModel:         "configured-gpt",
			wantProvider:      imageAPIStyleOpenAI,
			wantConfiguration: "tenant-a:image_gpt_image_2:v1",
		},
		{
			name:              "banana alias uses configured nanobanana model and verified async style",
			routingKey:        "nano-banana-fast",
			config:            &openaiclient.ClientConfig{APIKey: "secret-banana", BaseURL: "https://grsai.example.test/v1", Model: "configured-banana", APIStyle: "nanobanana"},
			wantClientName:    listingKitImageClientNameNanobanana,
			wantModel:         "configured-banana",
			wantProvider:      imageAPIStyleGRSAI,
			wantAsync:         true,
			wantConfiguration: "tenant-a:image_nanobanana:v1",
		},
		{
			name:              "custom selector preserves requested model",
			routingKey:        "custom-image-model",
			config:            &openaiclient.ClientConfig{APIKey: "secret-custom", BaseURL: "https://gemini.example.test/v1", Model: "configured-but-unused", APIStyle: imageAPIStyleGemini},
			wantClientName:    listingKitImageClientNameNanobanana,
			wantModel:         "custom-image-model",
			wantProvider:      imageAPIStyleGemini,
			wantConfiguration: "tenant-a:image_nanobanana:v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &studioCapabilityCatalogResolver{resolved: &openaiclient.ResolvedClientConfig{CacheKey: tt.wantConfiguration, Config: tt.config}}
			router := BuildStudioAICapabilityRouter(resolver)
			ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})

			decision, err := router.Decide(ctx, aicapability.RouteRequest{
				TenantID:            "tenant-a",
				UserID:              "user-a",
				Capability:          aicapability.CapabilityListingKitStudioImage,
				Operation:           aicapability.OperationImageGenerate,
				RequestedRoutingKey: tt.routingKey,
				RequiredFeatures:    []aicapability.ModelFeature{aicapability.FeatureImageGenerate},
			})
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if resolver.lastName != tt.wantClientName {
				t.Fatalf("resolver client name = %q, want %q", resolver.lastName, tt.wantClientName)
			}
			if decision.ModelID != tt.wantModel || decision.ProviderID != tt.wantProvider || decision.ConfigurationVersion != tt.wantConfiguration {
				t.Fatalf("decision = %+v", decision)
			}
			if tt.wantAsync {
				asyncDecision, err := router.Decide(ctx, aicapability.RouteRequest{TenantID: "tenant-a", Capability: aicapability.CapabilityListingKitStudioImage, Operation: aicapability.OperationAsyncImageGenerate, RequestedRoutingKey: tt.routingKey, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureAsyncImageJob}})
				if err != nil || asyncDecision.ModelID != tt.wantModel {
					t.Fatalf("async Decide() = %+v, %v", asyncDecision, err)
				}
			} else if _, err := router.Decide(ctx, aicapability.RouteRequest{TenantID: "tenant-a", Capability: aicapability.CapabilityListingKitStudioImage, Operation: aicapability.OperationAsyncImageGenerate, RequestedRoutingKey: tt.routingKey, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureAsyncImageJob}}); aicapability.CategoryOf(err) != aicapability.ErrorCapabilityUnavailable {
				t.Fatalf("async error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorCapabilityUnavailable)
			}
			if decision.ProviderID == "secret-default" || decision.ModelID == "secret-default" || decision.CredentialReference == "secret-default" {
				t.Fatalf("decision exposes API key: %+v", decision)
			}
		})
	}
}

func TestStudioCapabilityCatalogRejectsUnavailableCredentials(t *testing.T) {
	tests := []struct {
		name     string
		resolver openaiclient.ClientConfigResolver
	}{
		{name: "nil resolver"},
		{name: "missing resolved config", resolver: &studioCapabilityCatalogResolver{}},
		{name: "disabled config", resolver: &studioCapabilityCatalogResolver{resolved: &openaiclient.ResolvedClientConfig{Config: &openaiclient.ClientConfig{APIKey: "key", BaseURL: "https://example.test/v1"}}}},
		{name: "resolver error", resolver: &studioCapabilityCatalogResolver{err: errors.New("credential lookup failed")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildStudioAICapabilityRouter(tt.resolver).Decide(context.Background(), aicapability.RouteRequest{TenantID: "tenant-a", Capability: aicapability.CapabilityListingKitStudioImage, Operation: aicapability.OperationImageGenerate, RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureImageGenerate}})
			if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
				t.Fatalf("error category = %q, want %q (err %v)", aicapability.CategoryOf(err), aicapability.ErrorCredentialUnavailable, err)
			}
		})
	}
}

func TestResolveListingKitImageRoutePreservesStudioSelectorMapping(t *testing.T) {
	tests := []struct {
		name           string
		selector       string
		hasResolver    bool
		wantRoutingKey string
		wantCredential string
		wantConfigured bool
	}{
		{name: "blank defaults to GPT image", hasResolver: true, wantRoutingKey: "gpt-image-2", wantCredential: "image_gpt_image_2", wantConfigured: true},
		{name: "gpt image selector uses configured model", selector: " gpt-image-2 ", hasResolver: true, wantRoutingKey: "gpt-image-2", wantCredential: "image_gpt_image_2", wantConfigured: true},
		{name: "banana alias uses nanobanana credential", selector: "nano-banana-fast", hasResolver: true, wantRoutingKey: "nanobanana", wantCredential: "image_nanobanana", wantConfigured: true},
		{name: "custom selector with resolver uses nanobanana credential", selector: "custom-image-model", hasResolver: true, wantRoutingKey: "custom-image-model", wantCredential: "image_nanobanana"},
		{name: "custom selector without resolver uses image credential", selector: "custom-image-model", wantRoutingKey: "custom-image-model", wantCredential: "image"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveListingKitImageRoute(tt.selector, tt.hasResolver)
			if got.RoutingKey != tt.wantRoutingKey || got.CredentialReference != tt.wantCredential || got.UsesConfiguredModel != tt.wantConfigured {
				t.Fatalf("route = %+v", got)
			}
		})
	}
}

type studioCapabilityCatalogResolver struct {
	resolved *openaiclient.ResolvedClientConfig
	err      error
	lastName string
}

func (r *studioCapabilityCatalogResolver) ResolveClientConfig(ctx context.Context, clientName string, fallback *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	if fallback != nil {
		return nil, errors.New("catalog must not supply a fallback credential")
	}
	if identity := openaiclient.IdentityFromContext(ctx); identity.TenantID != "tenant-a" && r.resolved != nil {
		return nil, errors.New("tenant identity was not preserved")
	}
	r.lastName = clientName
	if r.err != nil {
		return nil, r.err
	}
	return r.resolved, nil
}
