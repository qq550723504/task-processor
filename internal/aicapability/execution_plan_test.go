package aicapability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutionPlanValidateAcceptsActiveAndLegacyPlans(t *testing.T) {
	active := ExecutionPlan{
		Mode: RoutingModeActive, RouteOutcome: RouteOutcomeActive,
		Decision: RouteDecision{
			Capability: CapabilityProductEnrichText,
			Operation:  OperationProductEnrichTextExtract,
			ProviderID: "openai", ModelID: "gpt", RoutingKey: "fast", CredentialReference: "fast", ConfigurationVersion: "config-v1",
		},
	}
	require.NoError(t, active.Validate())

	legacy := ExecutionPlan{
		Mode: RoutingModeLegacy, RouteOutcome: RouteOutcomeLegacy,
		LegacyClients: []string{"fast", "default"},
	}
	require.NoError(t, legacy.Validate())
}

func TestExecutionPlanValidateRejectsDeniedOrUnboundExecutablePlan(t *testing.T) {
	require.Error(t, (ExecutionPlan{Mode: RoutingModeActive}).Validate())
	require.Error(t, (ExecutionPlan{Mode: RoutingModeLegacy}).Validate())
	require.Error(t, (ExecutionPlan{
		Mode: RoutingModeActive, RouteOutcome: RouteOutcomeActive,
		Decision: RouteDecision{
			Capability: CapabilityProductEnrichText,
			Operation:  OperationProductEnrichTextExtract,
			ProviderID: "openai", ModelID: "gpt", RoutingKey: "fast",
		},
	}).Validate())
	require.Error(t, (ExecutionPlan{
		Mode: RoutingModeActive, RouteOutcome: RouteOutcomeActive,
		Decision: RouteDecision{
			Capability: CapabilityProductEnrichText,
			Operation:  OperationProductEnrichTextExtract,
			ProviderID: "openai", ModelID: "gpt", RoutingKey: "fast", CredentialReference: "fast",
		},
	}).Validate())
	require.Error(t, (ExecutionPlan{
		Mode: RoutingModeLegacy, RouteOutcome: RouteOutcomeLegacy,
		LegacyClients: []string{" ", "\t"},
	}).Validate())
}

func TestRouteRequestContractValidatesIdentityShapeAndFeatures(t *testing.T) {
	contract := RouteRequestContract{
		RequireTenantID: true,
		RequireUserID:   true,
		Capability:      CapabilityProductEnrichText,
		Operations:      []Operation{OperationProductEnrichTextExtract},
		RequiredFeatures: []ModelFeature{
			FeatureTextGenerate,
		},
	}
	valid := RouteRequest{
		TenantID: " tenant-a ", UserID: " user-a ", Capability: CapabilityProductEnrichText,
		Operation: OperationProductEnrichTextExtract, RequiredFeatures: []ModelFeature{FeatureTextGenerate},
	}
	require.NoError(t, contract.Validate(valid))

	tests := []struct {
		name     string
		mutate   func(*RouteRequest)
		category ErrorCategory
	}{
		{name: "blank tenant", mutate: func(request *RouteRequest) { request.TenantID = " " }, category: ErrorInvalidInput},
		{name: "blank user", mutate: func(request *RouteRequest) { request.UserID = " " }, category: ErrorInvalidInput},
		{name: "blank capability", mutate: func(request *RouteRequest) { request.Capability = "" }, category: ErrorInvalidInput},
		{name: "wrong capability", mutate: func(request *RouteRequest) { request.Capability = CapabilityProductEnrichVision }, category: ErrorPolicyDenied},
		{name: "blank operation", mutate: func(request *RouteRequest) { request.Operation = "" }, category: ErrorInvalidInput},
		{name: "wrong operation", mutate: func(request *RouteRequest) { request.Operation = OperationProductEnrichImageAnalyze }, category: ErrorPolicyDenied},
		{name: "missing required feature", mutate: func(request *RouteRequest) { request.RequiredFeatures = nil }, category: ErrorPolicyDenied},
		{name: "unexpected feature", mutate: func(request *RouteRequest) {
			request.RequiredFeatures = []ModelFeature{FeatureTextGenerate, FeatureVisionAnalyze}
		}, category: ErrorPolicyDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := valid
			request.RequiredFeatures = append([]ModelFeature(nil), valid.RequiredFeatures...)
			tt.mutate(&request)
			err := contract.Validate(request)
			require.Equal(t, tt.category, CategoryOf(err))
		})
	}
}
