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
			ProviderID: "openai", ModelID: "gpt", RoutingKey: "fast", CredentialReference: "fast",
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
		Mode: RoutingModeLegacy, RouteOutcome: RouteOutcomeLegacy,
		LegacyClients: []string{" ", "\t"},
	}).Validate())
}
