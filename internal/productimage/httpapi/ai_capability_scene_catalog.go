package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const productImageSceneRoutingKey = "productimage-image"

// BuildProductImageSceneCapabilityRouter exposes the existing image client
// configuration as a provider-neutral route for the new scene capability.
func BuildProductImageSceneCapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productImageSceneModelCatalog{resolver: resolver},
		productImageScenePolicyResolver{},
	)
}

type productImageSceneModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

func (c *productImageSceneModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productImageSceneRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, "image", nil)
	if err != nil || resolved == nil || resolved.Config == nil {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", err)
	}
	configured := resolved.Config
	if strings.TrimSpace(configured.APIKey) == "" || strings.TrimSpace(configured.BaseURL) == "" || strings.TrimSpace(configured.Model) == "" {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	providerID := strings.ToLower(strings.TrimSpace(configured.APIStyle))
	if providerID == "" || providerID == "openai" || providerID == "openai-compatible" {
		providerID = "openai"
	} else {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, "", nil)
	}
	return aicapability.ModelDefinition{
		ProviderID:           providerID,
		ModelID:              strings.TrimSpace(configured.Model),
		RoutingKey:           productImageSceneRoutingKey,
		CredentialReference:  "image",
		Features:             []aicapability.ModelFeature{aicapability.FeatureImageGenerate, aicapability.FeatureImageEdit},
		SupportsAsync:        false,
		Enabled:              true,
		ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productImageScenePolicyResolver struct{}

func (productImageScenePolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	return aicapability.TenantModelPolicy{
		TenantID:                   strings.TrimSpace(request.TenantID),
		Capability:                 aicapability.CapabilityProductImageScene,
		PreferredRoutingKeys:       []string{productImageSceneRoutingKey},
		AllowCrossProviderFallback: false,
		Version:                    "productimage-scene-v1",
	}, nil
}
