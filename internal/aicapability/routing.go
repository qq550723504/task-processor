package aicapability

import (
	"context"
	"strings"
)

type RouteRequest struct {
	TenantID            string
	UserID              string
	Capability          Capability
	Operation           Operation
	RequestedRoutingKey string
	RequiredFeatures    []ModelFeature
	IdempotencyKey      string
	TraceID             string
}

type ModelCatalog interface {
	ResolveModel(ctx context.Context, routingKey string) (ModelDefinition, error)
}

type PolicyResolver interface {
	ResolvePolicy(ctx context.Context, request RouteRequest) (TenantModelPolicy, error)
}

type Router interface {
	Decide(ctx context.Context, request RouteRequest) (RouteDecision, error)
}

type RouteDecision struct {
	Capability           Capability
	Operation            Operation
	ProviderID           string
	ModelID              string
	RoutingKey           string
	CredentialReference  string
	PolicyVersion        string
	ConfigurationVersion string
	FallbackIndex        int
	Reason               string
}

type PolicyRouter struct {
	catalog  ModelCatalog
	policies PolicyResolver
}

func NewPolicyRouter(catalog ModelCatalog, policies PolicyResolver) *PolicyRouter {
	return &PolicyRouter{catalog: catalog, policies: policies}
}

func (r *PolicyRouter) Decide(ctx context.Context, request RouteRequest) (RouteDecision, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.Capability = Capability(strings.TrimSpace(string(request.Capability)))
	request.Operation = Operation(strings.TrimSpace(string(request.Operation)))
	request.RequestedRoutingKey = strings.TrimSpace(request.RequestedRoutingKey)
	if request.TenantID == "" || request.Capability == "" || request.Operation == "" {
		return RouteDecision{}, NewError(ErrorInvalidInput, string(request.Operation), nil)
	}
	if r.catalog == nil || r.policies == nil {
		return RouteDecision{}, NewError(ErrorCapabilityUnavailable, string(request.Operation), nil)
	}

	policy, err := r.policies.ResolvePolicy(ctx, request)
	if err != nil {
		return RouteDecision{}, NewError(ErrorPolicyDenied, string(request.Operation), err)
	}
	if strings.TrimSpace(policy.TenantID) != "" && strings.TrimSpace(policy.TenantID) != request.TenantID {
		return RouteDecision{}, NewError(ErrorPolicyDenied, string(request.Operation), nil)
	}
	if capability := Capability(strings.TrimSpace(string(policy.Capability))); capability != "" && capability != request.Capability {
		return RouteDecision{}, NewError(ErrorPolicyDenied, string(request.Operation), nil)
	}

	candidates, err := routingCandidates(request.RequestedRoutingKey, policy)
	if err != nil {
		return RouteDecision{}, NewError(ErrorPolicyDenied, string(request.Operation), err)
	}
	if len(candidates) == 0 {
		return RouteDecision{}, NewError(ErrorCapabilityUnavailable, string(request.Operation), nil)
	}

	var lastErr error
	for index, key := range candidates {
		model, resolveErr := r.catalog.ResolveModel(ctx, key)
		if resolveErr != nil {
			lastErr = resolveErr
		} else if decision, valid := validateModel(request, policy, model, key, index); valid {
			return decision, nil
		} else {
			lastErr = NewError(ErrorCapabilityUnavailable, string(request.Operation), nil)
		}
		if !policy.AllowCrossProviderFallback {
			break
		}
	}
	if lastErr != nil {
		if category := CategoryOf(lastErr); category != ErrorUnknown {
			return RouteDecision{}, NewError(category, string(request.Operation), lastErr)
		}
	}
	return RouteDecision{}, NewError(ErrorCapabilityUnavailable, string(request.Operation), lastErr)
}

func routingCandidates(requested string, policy TenantModelPolicy) ([]string, error) {
	allowed := normalizedKeys(policy.AllowedRoutingKeys)
	preferred := normalizedKeys(policy.PreferredRoutingKeys)
	if requested != "" {
		if len(allowed) == 0 || contains(allowed, requested) {
			candidates := []string{requested}
			if policy.AllowCrossProviderFallback {
				for _, key := range preferred {
					if (len(allowed) == 0 || contains(allowed, key)) && key != requested {
						candidates = append(candidates, key)
					}
				}
			}
			return candidates, nil
		}
	}
	for _, key := range preferred {
		if len(allowed) == 0 || contains(allowed, key) {
			return candidatesFromPreferred(key, preferred, allowed, policy.AllowCrossProviderFallback), nil
		}
	}
	return nil, nil
}

func candidatesFromPreferred(first string, preferred, allowed []string, allowFallback bool) []string {
	candidates := []string{first}
	if !allowFallback {
		return candidates
	}
	for _, key := range preferred {
		if key != first && (len(allowed) == 0 || contains(allowed, key)) {
			candidates = append(candidates, key)
		}
	}
	return candidates
}

func validateModel(request RouteRequest, policy TenantModelPolicy, model ModelDefinition, selectedRoutingKey string, fallbackIndex int) (RouteDecision, bool) {
	if !model.Enabled {
		return RouteDecision{}, false
	}
	credentialReference := strings.TrimSpace(model.CredentialReference)
	if credentialReference == "" {
		credentialReference = strings.TrimSpace(policy.CredentialReference)
	}
	if !supportsAll(model.Features, request.RequiredFeatures) || !supportsTags(model.DataPolicyTags, policy.RequiredDataPolicyTags) {
		return RouteDecision{}, false
	}
	routingKey := strings.TrimSpace(model.RoutingKey)
	if routingKey == "" {
		routingKey = selectedRoutingKey
	}
	return RouteDecision{
		Capability: request.Capability, Operation: request.Operation, ProviderID: strings.TrimSpace(model.ProviderID), ModelID: strings.TrimSpace(model.ModelID), RoutingKey: routingKey, CredentialReference: credentialReference, PolicyVersion: strings.TrimSpace(policy.Version), ConfigurationVersion: strings.TrimSpace(model.ConfigurationVersion), FallbackIndex: fallbackIndex, Reason: "policy_router",
	}, true
}

func normalizedKeys(keys []string) []string {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" && !contains(result, key) {
			result = append(result, key)
		}
	}
	return result
}

func supportsAll(available, required []ModelFeature) bool {
	for _, feature := range required {
		if !containsFeature(available, feature) {
			return false
		}
	}
	return true
}

func supportsTags(available, required []string) bool {
	for _, tag := range normalizedKeys(required) {
		if !contains(normalizedKeys(available), tag) {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsFeature(values []ModelFeature, wanted ModelFeature) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
