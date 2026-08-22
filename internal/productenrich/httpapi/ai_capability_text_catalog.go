package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const (
	productEnrichTextRoutingKey    = "productenrich-text"
	productEnrichTextClientName    = "fast"
	productEnrichVisionRoutingKey  = "productenrich-vision"
	productEnrichVisionClientName  = "vision"
	productEnrichListingRoutingKey = "productenrich-listing"
	productEnrichListingClientName = "default"
	productEnrichFusionRoutingKey  = productEnrichListingRoutingKey
	productEnrichFusionClientName  = productEnrichListingClientName
)

// BuildProductEnrichTextCapabilityRouter maps the existing tenant-aware fast
// client to the provider-neutral ProductEnrich text capability.
func BuildProductEnrichTextCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver, clientName: productEnrichTextClientName},
		productEnrichTextPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

func BuildProductEnrichTextQualityCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string, clientName string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver, clientName: strings.TrimSpace(clientName)},
		productEnrichTextPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

// BuildProductEnrichFusionCapabilityRouter routes multimodal representation
// fusion through the same tenant policy and default-client boundary as listing
// generation, while keeping a distinct capability/operation in the ledger.
func BuildProductEnrichFusionCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichFusionModelCatalog{resolver: resolver},
		productEnrichFusionPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

// BuildProductEnrichVisionCapabilityRouter maps the existing tenant-aware
// vision client to the provider-neutral ProductEnrich vision capability.
func BuildProductEnrichVisionCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver, clientName: productEnrichVisionClientName},
		productEnrichVisionPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

func BuildProductEnrichVisionQualityCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string, clientName string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver, clientName: strings.TrimSpace(clientName)},
		productEnrichVisionPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
	)
}

// BuildProductEnrichListingCapabilityRouter maps the existing default client
// to the provider-neutral primary listing JSON generation capability.
func BuildProductEnrichListingCapabilityRouter(resolver openaiclient.ClientConfigResolver, allowedTenantIDs []string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichListingModelCatalog{resolver: resolver},
		productEnrichListingPolicyResolver{allowedTenantIDs: productEnrichTextTenantIDSet(allowedTenantIDs)},
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
	resolver   openaiclient.ClientConfigResolver
	clientName string
}

func (c *productEnrichTextModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichTextRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, c.clientName, nil)
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
		CredentialReference: c.clientName, Features: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichTextPolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

type productEnrichVisionModelCatalog struct {
	resolver   openaiclient.ClientConfigResolver
	clientName string
}

func (c *productEnrichVisionModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichVisionRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, c.clientName, nil)
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
		CredentialReference: c.clientName, Features: []aicapability.ModelFeature{aicapability.FeatureVisionAnalyze},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichVisionPolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

type productEnrichListingModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

type productEnrichFusionModelCatalog struct {
	resolver openaiclient.ClientConfigResolver
}

func (c *productEnrichFusionModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichFusionRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, productEnrichFusionClientName, nil)
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
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichFusionRoutingKey,
		CredentialReference: productEnrichFusionClientName, Features: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

func (c *productEnrichListingModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichListingRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := c.resolver.ResolveClientConfig(ctx, productEnrichListingClientName, nil)
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
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichListingRoutingKey,
		CredentialReference: productEnrichListingClientName, Features: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichListingPolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

type productEnrichFusionPolicyResolver struct {
	allowedTenantIDs map[string]struct{}
}

func (r productEnrichFusionPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if _, ok := r.allowedTenantIDs[strings.TrimSpace(request.TenantID)]; !ok {
		return aicapability.TenantModelPolicy{}, aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichFusion,
		PreferredRoutingKeys: []string{productEnrichFusionRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-fusion-v1",
	}, nil
}

func (r productEnrichListingPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if _, ok := r.allowedTenantIDs[strings.TrimSpace(request.TenantID)]; !ok {
		return aicapability.TenantModelPolicy{}, aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichListing,
		PreferredRoutingKeys: []string{productEnrichListingRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-listing-v1",
	}, nil
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
