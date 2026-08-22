package httpapi

import (
	"context"
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
func BuildProductEnrichTextCapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver, clientName: productEnrichTextClientName},
		productEnrichTextPolicyResolver{},
	)
}

func BuildProductEnrichTextQualityCapabilityRouter(resolver openaiclient.ClientConfigResolver, clientName string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichTextModelCatalog{resolver: resolver, clientName: strings.TrimSpace(clientName)},
		productEnrichTextPolicyResolver{},
	)
}

// BuildProductEnrichFusionCapabilityRouter routes multimodal representation
// fusion through the same tenant policy and default-client boundary as listing
// generation, while keeping a distinct capability/operation in the ledger.
func BuildProductEnrichFusionCapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichFusionModelCatalog{resolver: resolver},
		productEnrichFusionPolicyResolver{},
	)
}

// BuildProductEnrichVisionCapabilityRouter maps the existing tenant-aware
// vision client to the provider-neutral ProductEnrich vision capability.
func BuildProductEnrichVisionCapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver, clientName: productEnrichVisionClientName},
		productEnrichVisionPolicyResolver{},
	)
}

func BuildProductEnrichVisionQualityCapabilityRouter(resolver openaiclient.ClientConfigResolver, clientName string) aicapability.Router {
	return aicapability.NewPolicyRouter(
		&productEnrichVisionModelCatalog{resolver: resolver, clientName: strings.TrimSpace(clientName)},
		productEnrichVisionPolicyResolver{},
	)
}

// BuildProductEnrichListingCapabilityRouter maps the existing default client
// to the provider-neutral primary listing JSON generation capability.
func BuildProductEnrichListingCapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
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

func BuildProductEnrichLegacyRouteMetadataResolver(resolver openaiclient.ClientConfigResolver) productenrichenrich.LegacyRouteMetadataResolver {
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
	resolver openaiclient.ClientConfigResolver
}

func (r productEnrichLegacyRouteMetadataResolver) ResolveLegacyRoute(ctx context.Context, clientName string) (aicapability.RouteDecision, error) {
	clientName = strings.TrimSpace(clientName)
	if r.resolver == nil || clientName == "" {
		return aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	resolved, err := r.resolver.ResolveClientConfig(ctx, clientName, nil)
	if err != nil || resolved == nil || resolved.Config == nil || strings.TrimSpace(resolved.CacheKey) == "" {
		return aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", err)
	}
	configured := resolved.Config
	if strings.TrimSpace(configured.APIKey) == "" || strings.TrimSpace(configured.BaseURL) == "" || strings.TrimSpace(configured.Model) == "" {
		return aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCredentialUnavailable, "", nil)
	}
	providerID, ok := productEnrichProviderID(configured.APIStyle)
	if !ok {
		return aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, "", nil)
	}
	return aicapability.RouteDecision{
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: clientName,
		CredentialReference: clientName, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
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
	providerID, ok := productEnrichProviderID(configured.APIStyle)
	if !ok {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, "", nil)
	}
	return aicapability.ModelDefinition{
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichTextRoutingKey,
		CredentialReference: c.clientName, Features: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichTextPolicyResolver struct{}

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
	providerID, ok := productEnrichProviderID(configured.APIStyle)
	if !ok {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, "", nil)
	}
	return aicapability.ModelDefinition{
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichVisionRoutingKey,
		CredentialReference: c.clientName, Features: []aicapability.ModelFeature{aicapability.FeatureVisionAnalyze},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

type productEnrichVisionPolicyResolver struct{}

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
	providerID, ok := productEnrichProviderID(configured.APIStyle)
	if !ok {
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
	providerID, ok := productEnrichProviderID(configured.APIStyle)
	if !ok {
		return aicapability.ModelDefinition{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, "", nil)
	}
	return aicapability.ModelDefinition{
		ProviderID: providerID, ModelID: strings.TrimSpace(configured.Model), RoutingKey: productEnrichListingRoutingKey,
		CredentialReference: productEnrichListingClientName, Features: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		Enabled: true, ConfigurationVersion: strings.TrimSpace(resolved.CacheKey),
	}, nil
}

// productEnrichProviderID normalizes credential protocol metadata for the
// provider-neutral ledger. Gemini credentials are still served by the
// tenant-aware chat manager, but must retain their provider identity instead
// of being rejected as an unsupported catalog style.
func productEnrichProviderID(apiStyle string) (string, bool) {
	switch style := strings.ToLower(strings.TrimSpace(apiStyle)); style {
	case "", "openai", "openai-compatible":
		return "openai", true
	case "gemini":
		return "gemini", true
	default:
		return "", false
	}
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
