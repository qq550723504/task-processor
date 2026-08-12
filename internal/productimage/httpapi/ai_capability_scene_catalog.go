package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const (
	productImageSceneRoutingKey = "productimage-image"
	productImageSceneClientName = "image_gpt_image_2"
)

// BuildProductImageSceneCapabilityRouter exposes the existing image client
// configuration as a provider-neutral route for the new scene capability.
func BuildProductImageSceneCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs ...[]string) aicapability.Router {
	var tenants []string
	if len(allowedTenantIDs) > 0 {
		tenants = allowedTenantIDs[0]
	}
	return aicapability.NewPolicyRouter(
		&productImageSceneModelCatalog{resolver: resolver},
		productImageScenePolicyResolver{allowedTenantIDs: tenantIDSet(tenants)},
	)
}

type productImageSceneModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

func (c *productImageSceneModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productImageSceneRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, productImageSceneClientName, nil)
	if err != nil || resolved == nil || resolved.Config == nil || strings.TrimSpace(resolved.CacheKey) == "" {
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
		CredentialReference:  productImageSceneClientName,
		Features:             []aicapability.ModelFeature{aicapability.FeatureImageGenerate, aicapability.FeatureImageEdit},
		SupportsAsync:        false,
		Enabled:              true,
		ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productImageScenePolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

func (r productImageScenePolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if _, ok := r.allowedTenantIDs[strings.TrimSpace(request.TenantID)]; !ok {
		return aicapability.TenantModelPolicy{}, aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
	}
	return aicapability.TenantModelPolicy{
		TenantID:                   strings.TrimSpace(request.TenantID),
		Capability:                 aicapability.CapabilityProductImageScene,
		PreferredRoutingKeys:       []string{productImageSceneRoutingKey},
		AllowCrossProviderFallback: false,
		Version:                    "productimage-scene-v1",
	}, nil
}

func tenantIDSet(ids []string) map[string]struct{} {
	result := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if normalized := strings.TrimSpace(id); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}
