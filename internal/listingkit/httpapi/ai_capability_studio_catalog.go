package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

// BuildStudioAICapabilityRouter exposes the existing Studio credential
// selection as a provider-neutral capability routing decision.
func BuildStudioAICapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&studioCredentialModelCatalog{resolver: resolver},
		studioLegacyPolicyResolver{},
	)
}

type studioCredentialModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

func (c *studioCredentialModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	route := resolveListingKitImageRoute(routingKey, true)
	resolved, err := c.resolver.ResolveClientConfig(ctx, route.CredentialReference, nil)
	if err != nil || resolved == nil || resolved.Config == nil {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", err)
	}
	configured := resolved.Config
	if strings.TrimSpace(configured.APIKey) == "" || strings.TrimSpace(configured.BaseURL) == "" || strings.TrimSpace(configured.Model) == "" {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	modelID := route.RoutingKey
	if route.UsesConfiguredModel {
		modelID = strings.TrimSpace(configured.Model)
	}
	features := []aicapability.ModelFeature{aicapability.FeatureImageGenerate, aicapability.FeatureImageEdit}
	style := normalizeImageAPIStyle(configured)
	if route.CredentialReference == listingKitImageClientNameNanobanana {
		style = normalizeNanobananaImageAPIStyle(configured)
	}
	supportsAsync := style == imageAPIStyleGRSAI
	if supportsAsync {
		features = append(features, aicapability.FeatureAsyncImageJob)
	}
	return aicapability.ModelDefinition{
		ProviderID:           style,
		ModelID:              modelID,
		RoutingKey:           route.RoutingKey,
		CredentialReference:  route.CredentialReference,
		Features:             features,
		SupportsAsync:        supportsAsync,
		Enabled:              true,
		ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type studioLegacyPolicyResolver struct{}

func (studioLegacyPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	return aicapability.TenantModelPolicy{
		TenantID:                   strings.TrimSpace(request.TenantID),
		Capability:                 aicapability.CapabilityListingKitStudioImage,
		PreferredRoutingKeys:       []string{listingKitImageModelSelectorGPTImage2},
		AllowCrossProviderFallback: false,
		Version:                    "listingkit-studio-legacy-v1",
	}, nil
}
