package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const (
	productEnrichTextRoutingKey   = "productenrich-text"
	productEnrichTextClientName   = "fast"
	productEnrichVisionRoutingKey = "productenrich-vision"
	productEnrichVisionClientName = "vision"
)

// BuildProductEnrichTextCapabilityRouter maps the existing tenant-aware fast
// client to the provider-neutral ProductEnrich text capability.
func BuildProductEnrichTextCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver},
		productEnrichTextPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

// BuildProductEnrichVisionCapabilityRouter maps the existing tenant-aware
// vision client to the provider-neutral ProductEnrich vision capability.
func BuildProductEnrichVisionCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver},
		productEnrichVisionPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

func productEnrichTextTenantIDSet(ids []string) map[string]struct{} {
	result := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			result[id] = struct{}{}
		}
	}
	return result
}

type productEnrichTextModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

func (c *productEnrichTextModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichTextRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, productEnrichTextClientName, nil)
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
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichTextRoutingKey,
		CredentialReference: productEnrichTextClientName, Features: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichTextPolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

type productEnrichVisionModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

func (c *productEnrichVisionModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichVisionRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, productEnrichVisionClientName, nil)
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
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichVisionRoutingKey,
		CredentialReference: productEnrichVisionClientName, Features: []aicapability.ModelFeature{aicapability.FeatureVisionAnalyze},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichVisionPolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

func (r productEnrichVisionPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if _, ok := r.allowedTenantIDs[strings.TrimSpace(request.TenantID)]; !ok {
		return aicapability.TenantModelPolicy{}, aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichVision,
		PreferredRoutingKeys: []string{productEnrichVisionRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-vision-v1",
	}, nil
}

func (r productEnrichTextPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if _, ok := r.allowedTenantIDs[strings.TrimSpace(request.TenantID)]; !ok {
		return aicapability.TenantModelPolicy{}, aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichText,
		PreferredRoutingKeys: []string{productEnrichTextRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-text-v1",
	}, nil
}
