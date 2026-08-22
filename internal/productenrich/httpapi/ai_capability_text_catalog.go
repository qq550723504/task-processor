package httpapi

import (
	"context"
	"errors"
	"strings"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
	productenrichenrich "task-processor/internal/productenrich/enrich"
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
func BuildProductEnrichTextCapabilityRouter(resolver openaiclient.EffectiveClientRouteResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver, clientName: productEnrichTextClientName},
		productEnrichTextPolicyResolver{},
	)
}

func BuildProductEnrichTextQualityCapabilityRouter(resolver openaiclient.EffectiveClientRouteResolver, clientName string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver, clientName: strings.TrimSpace(clientName)},
		productEnrichTextPolicyResolver{},
	)
}

// BuildProductEnrichFusionCapabilityRouter routes multimodal representation
// fusion through the same tenant policy and default-client boundary as listing
// generation, while keeping a distinct capability/operation in the ledger.
func BuildProductEnrichFusionCapabilityRouter(resolver openaiclient.EffectiveClientRouteResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichFusionModelCatalog{resolver: resolver},
		productEnrichFusionPolicyResolver{},
	)
}

// BuildProductEnrichVisionCapabilityRouter maps the existing tenant-aware
// vision client to the provider-neutral ProductEnrich vision capability.
func BuildProductEnrichVisionCapabilityRouter(resolver openaiclient.EffectiveClientRouteResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver, clientName: productEnrichVisionClientName},
		productEnrichVisionPolicyResolver{},
	)
}

func BuildProductEnrichVisionQualityCapabilityRouter(resolver openaiclient.EffectiveClientRouteResolver, clientName string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver, clientName: strings.TrimSpace(clientName)},
		productEnrichVisionPolicyResolver{},
	)
}

// BuildProductEnrichListingCapabilityRouter maps the existing default client
// to the provider-neutral primary listing JSON generation capability.
func BuildProductEnrichListingCapabilityRouter(resolver openaiclient.EffectiveClientRouteResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichListingModelCatalog{resolver: resolver},
		productEnrichListingPolicyResolver{},
	)
}

func BuildProductEnrichExecutionPlanner(router aicapability.Router, activeTenantIDs []string, legacyClients []string) aicapability.ExecutionPlanner {
	return tenantRolloutPlanner{
		router: router, activeTenantIDs: productEnrichTextTenantIDSet(activeTenantIDs),
		legacyClients: append([]string(nil), legacyClients...),
	}
}

func BuildProductEnrichLegacyRouteMetadataResolver(resolver openaiclient.EffectiveClientRouteResolver) productenrichenrich.LegacyRouteMetadataResolver {
	return productEnrichLegacyRouteMetadataResolver{resolver: resolver}
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

type tenantRolloutPlanner struct {
	router          aicapability.Router
	activeTenantIDs map[string]struct{}
	legacyClients   []string
}

func (p tenantRolloutPlanner) Plan(ctx context.Context, request aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
	if _, active := p.activeTenantIDs[strings.TrimSpace(request.TenantID)]; !active {
		plan := aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy,
			LegacyClients: append([]string(nil), p.legacyClients...),
		}
		return plan, plan.Validate()
	}
	if p.router == nil {
		return aicapability.ExecutionPlan{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(request.Operation), nil)
	}
	decision, err := p.router.Decide(ctx, request)
	if err != nil {
		return aicapability.ExecutionPlan{}, err
	}
	plan := aicapability.ExecutionPlan{
		Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive, Decision: decision,
	}
	return plan, plan.Validate()
}

type productEnrichLegacyRouteMetadataResolver struct {
	resolver openaiclient.EffectiveClientRouteResolver
}

func (r productEnrichLegacyRouteMetadataResolver) ResolveLegacyRoute(ctx context.Context, clientName string) (aicapability.RouteDecision, error) {
	clientName = strings.TrimSpace(clientName)
	if r.resolver == nil || clientName == "" {
		return aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := r.resolver.ResolveEffectiveClientRoute(ctx, clientName)
	if errors.Is(err, openaiclient.ErrClientConfigurationUnavailable) {
		return aicapability.RouteDecision{}, productenrichenrich.ErrLegacyRouteUnavailable
	}
	if err != nil {
		return aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", err)
	}
	return aicapability.RouteDecision{
		ProviderID: resolved.ProviderID, ModelID: resolved.ModelID, RoutingKey: clientName,
		CredentialReference: resolved.CredentialReference, ConfigurationVersion: resolved.ConfigurationVersion,
	}, nil
}

type productEnrichTextModelCatalog struct {
	resolver   openaiclient.EffectiveClientRouteResolver
	clientName string
}

func (c *productEnrichTextModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichTextRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	return resolveProductEnrichModel(ctx, c.resolver, c.clientName, productEnrichTextRoutingKey, aicapability.FeatureTextGenerate)
}

type productEnrichTextPolicyResolver struct{}

type productEnrichVisionModelCatalog struct {
	resolver   openaiclient.EffectiveClientRouteResolver
	clientName string
}

func (c *productEnrichVisionModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichVisionRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	return resolveProductEnrichModel(ctx, c.resolver, c.clientName, productEnrichVisionRoutingKey, aicapability.FeatureVisionAnalyze)
}

type productEnrichVisionPolicyResolver struct{}

type productEnrichListingModelCatalog struct {
	resolver openaiclient.EffectiveClientRouteResolver
}

type productEnrichFusionModelCatalog struct {
	resolver openaiclient.EffectiveClientRouteResolver
}

func (c *productEnrichFusionModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichFusionRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	return resolveProductEnrichModel(ctx, c.resolver, productEnrichFusionClientName, productEnrichFusionRoutingKey, aicapability.FeatureTextGenerate)
}

func (c *productEnrichListingModelCatalog) ResolveModel(ctx context.Context, routingKey string) (aicapability.ModelDefinition, error) {
	if c == nil || c.resolver == nil || strings.TrimSpace(routingKey) != productEnrichListingRoutingKey {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	return resolveProductEnrichModel(ctx, c.resolver, productEnrichListingClientName, productEnrichListingRoutingKey, aicapability.FeatureTextGenerate)
}

func resolveProductEnrichModel(ctx context.Context, resolver openaiclient.EffectiveClientRouteResolver, clientName, routingKey string, feature aicapability.ModelFeature) (aicapability.ModelDefinition, error) {
	route, err := resolver.ResolveEffectiveClientRoute(ctx, clientName)
	if err != nil {
		category := aicapability.ErrorCredentialUnavailable
		if errors.Is(err, openaiclient.ErrClientConfigurationUnsupported) {
			category = aicapability.ErrorCapabilityUnavailable
		}
		return aicapability.ModelDefinition{}, aicapability.NewError(category, "", err)
	}
	if strings.TrimSpace(route.ProviderID) == "" || strings.TrimSpace(route.ModelID) == "" || strings.TrimSpace(route.CredentialReference) == "" || strings.TrimSpace(route.ConfigurationVersion) == "" {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	return aicapability.ModelDefinition{
		ProviderID: route.ProviderID, ModelID: route.ModelID, RoutingKey: routingKey,
		CredentialReference: route.CredentialReference, Features: []aicapability.ModelFeature{feature},
		Enabled: true, ConfigurationVersion: route.ConfigurationVersion,
	}, nil
}

type productEnrichListingPolicyResolver struct{}

type productEnrichFusionPolicyResolver struct{}

func (r productEnrichFusionPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if err := validateProductEnrichPolicyRequest(request, aicapability.CapabilityProductEnrichFusion, aicapability.OperationProductEnrichMultimodalFuse); err != nil {
		return aicapability.TenantModelPolicy{}, err
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichFusion,
		PreferredRoutingKeys: []string{productEnrichFusionRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-fusion-v1",
	}, nil
}

func (r productEnrichListingPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if err := validateProductEnrichPolicyRequest(request, aicapability.CapabilityProductEnrichListing,
		aicapability.OperationProductEnrichJSONGenerate,
		aicapability.OperationProductEnrichSpecsGenerate,
		aicapability.OperationProductEnrichVariantsGenerate,
	); err != nil {
		return aicapability.TenantModelPolicy{}, err
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichListing,
		PreferredRoutingKeys: []string{productEnrichListingRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-listing-v1",
	}, nil
}

func (r productEnrichVisionPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if err := validateProductEnrichPolicyRequest(request, aicapability.CapabilityProductEnrichVision,
		aicapability.OperationProductEnrichImageAnalyze,
		aicapability.OperationProductEnrichVisionQualityScore,
	); err != nil {
		return aicapability.TenantModelPolicy{}, err
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichVision,
		PreferredRoutingKeys: []string{productEnrichVisionRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-vision-v1",
	}, nil
}

func (r productEnrichTextPolicyResolver) ResolvePolicy(_ context.Context, request aicapability.RouteRequest) (aicapability.TenantModelPolicy, error) {
	if err := validateProductEnrichPolicyRequest(request, aicapability.CapabilityProductEnrichText,
		aicapability.OperationProductEnrichTextExtract,
		aicapability.OperationProductEnrichTextQualityScore,
	); err != nil {
		return aicapability.TenantModelPolicy{}, err
	}
	return aicapability.TenantModelPolicy{
		TenantID: strings.TrimSpace(request.TenantID), Capability: aicapability.CapabilityProductEnrichText,
		PreferredRoutingKeys: []string{productEnrichTextRoutingKey}, AllowCrossProviderFallback: false,
		Version: "productenrich-text-v1",
	}, nil
}

func validateProductEnrichPolicyRequest(request aicapability.RouteRequest, capability aicapability.Capability, operations ...aicapability.Operation) error {
	if strings.TrimSpace(request.TenantID) == "" || request.Capability != capability {
		return aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
	}
	for _, operation := range operations {
		if request.Operation == operation {
			return nil
		}
	}
	return aicapability.NewError(aicapability.ErrorPolicyDenied, string(request.Operation), nil)
}
