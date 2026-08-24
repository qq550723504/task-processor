package aicapability

import "strings"

// RouteRequestContract validates the provider-neutral shape a planner accepts
// before it selects an active or legacy execution route. RequiredFeatures is
// the exact normalized feature set for that planner shape.
type RouteRequestContract struct {
	RequireTenantID  bool
	RequireUserID    bool
	Capability       Capability
	Operations       []Operation
	RequiredFeatures []ModelFeature
}

func (c RouteRequestContract) Validate(request RouteRequest) error {
	operation := Operation(strings.TrimSpace(string(request.Operation)))
	capability := Capability(strings.TrimSpace(string(request.Capability)))
	if strings.TrimSpace(string(c.Capability)) == "" || len(normalizedOperations(c.Operations)) == 0 {
		return NewError(ErrorCapabilityUnavailable, string(operation), nil)
	}
	if (c.RequireTenantID && strings.TrimSpace(request.TenantID) == "") ||
		(c.RequireUserID && strings.TrimSpace(request.UserID) == "") ||
		capability == "" || operation == "" {
		return NewError(ErrorInvalidInput, string(operation), nil)
	}
	if capability != Capability(strings.TrimSpace(string(c.Capability))) || !containsOperation(normalizedOperations(c.Operations), operation) {
		return NewError(ErrorPolicyDenied, string(operation), nil)
	}
	if !sameFeatures(normalizedFeatures(c.RequiredFeatures), normalizedFeatures(request.RequiredFeatures)) {
		return NewError(ErrorPolicyDenied, string(operation), nil)
	}
	return nil
}

func normalizedOperations(operations []Operation) []Operation {
	result := make([]Operation, 0, len(operations))
	for _, operation := range operations {
		operation = Operation(strings.TrimSpace(string(operation)))
		if operation != "" && !containsOperation(result, operation) {
			result = append(result, operation)
		}
	}
	return result
}

func containsOperation(operations []Operation, wanted Operation) bool {
	for _, operation := range operations {
		if operation == wanted {
			return true
		}
	}
	return false
}

func normalizedFeatures(features []ModelFeature) []ModelFeature {
	result := make([]ModelFeature, 0, len(features))
	for _, feature := range features {
		feature = ModelFeature(strings.TrimSpace(string(feature)))
		if feature != "" && !containsFeature(result, feature) {
			result = append(result, feature)
		}
	}
	return result
}

func sameFeatures(left, right []ModelFeature) bool {
	if len(left) != len(right) {
		return false
	}
	for _, feature := range left {
		if !containsFeature(right, feature) {
			return false
		}
	}
	return true
}
