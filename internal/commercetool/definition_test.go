package commercetool

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validDefinition() Definition {
	return Definition{
		Ref:          ToolRef{ID: "product.canonical.inspect", Version: "v1.0.0"},
		Capability:   "product.canonical",
		Owner:        "product.catalog",
		Description:  "Inspect the authorized canonical product projection.",
		InputSchema:  json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["task_id"],"properties":{"task_id":{"type":"string","minLength":1}}}`),
		OutputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["task_id"],"properties":{"task_id":{"type":"string"}}}`),
		Risk:         RiskRead,
		Permission:   PermissionRequirement{Permission: "listingkit.admin.read"},
		SideEffects:  SideEffectPolicy{Mode: SideEffectNone},
		Idempotency:  IdempotencyPolicy{Mode: IdempotencyDeterministic},
		Timeout:      TimeoutPolicy{Duration: 2 * time.Second},
		Retry:        RetryPolicy{Owner: RetryOwnerCaller},
		Usage:        UsagePolicy{Owner: UsageOwnerUnmetered},
	}
}

func TestDefinitionValidateAcceptsCompleteReadTool(t *testing.T) {
	require.NoError(t, validDefinition().Validate())
}

func TestDefinitionValidateRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"tool ID", func(d *Definition) { d.Ref.ID = "" }},
		{"tool version", func(d *Definition) { d.Ref.Version = "" }},
		{"capability", func(d *Definition) { d.Capability = "" }},
		{"owner", func(d *Definition) { d.Owner = "" }},
		{"description", func(d *Definition) { d.Description = "" }},
		{"input schema", func(d *Definition) { d.InputSchema = nil }},
		{"output schema", func(d *Definition) { d.OutputSchema = nil }},
		{"risk", func(d *Definition) { d.Risk = "" }},
		{"permission", func(d *Definition) { d.Permission.Permission = "" }},
		{"side effects", func(d *Definition) { d.SideEffects.Mode = "" }},
		{"idempotency", func(d *Definition) { d.Idempotency.Mode = "" }},
		{"timeout", func(d *Definition) { d.Timeout.Duration = 0 }},
		{"retry owner", func(d *Definition) { d.Retry.Owner = "" }},
		{"usage owner", func(d *Definition) { d.Usage.Owner = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			tt.mutate(&definition)

			require.Error(t, definition.Validate())
		})
	}
}

func TestDefinitionValidateRejectsWhitespaceOnlyTextFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"description", func(d *Definition) { d.Description = " \t\n" }},
		{"input schema", func(d *Definition) { d.InputSchema = json.RawMessage(" \t\n") }},
		{"output schema", func(d *Definition) { d.OutputSchema = json.RawMessage(" \t\n") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			tt.mutate(&definition)

			require.Error(t, definition.Validate())
		})
	}
}

func TestDefinitionValidateRejectsInvalidQualifiedFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"capability", func(d *Definition) { d.Capability = "product/canonical" }},
		{"owner", func(d *Definition) { d.Owner = "product catalog" }},
		{"permission", func(d *Definition) { d.Permission.Permission = "listingkit:admin.read" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			tt.mutate(&definition)

			require.Error(t, definition.Validate())
		})
	}
}

func TestDefinitionValidateAcceptsRiskSideEffectIdempotencyMatrix(t *testing.T) {
	tests := []struct {
		name        string
		risk        RiskLevel
		sideEffects SideEffectMode
		idempotency IdempotencyMode
	}{
		{"read not applicable", RiskRead, SideEffectNone, IdempotencyNotApplicable},
		{"read deterministic", RiskRead, SideEffectNone, IdempotencyDeterministic},
		{"read required key", RiskRead, SideEffectNone, IdempotencyRequiredKey},
		{"compute not applicable", RiskCompute, SideEffectNone, IdempotencyNotApplicable},
		{"compute deterministic", RiskCompute, SideEffectNone, IdempotencyDeterministic},
		{"compute required key", RiskCompute, SideEffectNone, IdempotencyRequiredKey},
		{"propose not applicable", RiskPropose, SideEffectNone, IdempotencyNotApplicable},
		{"propose deterministic", RiskPropose, SideEffectNone, IdempotencyDeterministic},
		{"propose required key", RiskPropose, SideEffectNone, IdempotencyRequiredKey},
		{"write", RiskWrite, SideEffectBusinessMutation, IdempotencyRequiredKey},
		{"publish", RiskPublish, SideEffectExternalMutation, IdempotencyRequiredKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			definition.Risk = tt.risk
			definition.SideEffects.Mode = tt.sideEffects
			definition.Idempotency.Mode = tt.idempotency

			require.NoError(t, definition.Validate())
		})
	}
}

func TestDefinitionValidateRejectsRiskSideEffectIdempotencyCombinationsOutsideMatrix(t *testing.T) {
	tests := []struct {
		name        string
		risk        RiskLevel
		sideEffects SideEffectMode
		idempotency IdempotencyMode
	}{
		{"read business mutation", RiskRead, SideEffectBusinessMutation, IdempotencyDeterministic},
		{"compute external mutation", RiskCompute, SideEffectExternalMutation, IdempotencyNotApplicable},
		{"propose business mutation", RiskPropose, SideEffectBusinessMutation, IdempotencyRequiredKey},
		{"write without idempotency key", RiskWrite, SideEffectBusinessMutation, IdempotencyDeterministic},
		{"publish without idempotency key", RiskPublish, SideEffectExternalMutation, IdempotencyNotApplicable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			definition.Risk = tt.risk
			definition.SideEffects.Mode = tt.sideEffects
			definition.Idempotency.Mode = tt.idempotency

			require.Error(t, definition.Validate())
		})
	}
}

func TestToolAndAgentRefValidateRejectInvalidIdentifiers(t *testing.T) {
	invalidIDs := []string{"Product.canonical.inspect", "product canonical.inspect", "product/canonical.inspect", "product:canonical.inspect"}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			require.Error(t, (ToolRef{ID: id, Version: "v1.0.0"}).Validate())
			require.Error(t, (AgentRef{ID: id, Version: "v1.0.0"}).Validate())
		})
	}
}

func TestToolAndAgentRefValidateRejectNonCanonicalVersions(t *testing.T) {
	invalidVersions := []string{"v1", "1.0.0", "v1.0.0+build.1"}

	for _, version := range invalidVersions {
		t.Run(version, func(t *testing.T) {
			require.Error(t, (ToolRef{ID: "product.canonical.inspect", Version: version}).Validate())
			require.Error(t, (AgentRef{ID: "product.canonical.inspect", Version: version}).Validate())
		})
	}
}

func TestDefinitionValidateRejectsUnknownEnumValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"risk", func(d *Definition) { d.Risk = "unknown" }},
		{"side effects", func(d *Definition) { d.SideEffects.Mode = "unknown" }},
		{"idempotency", func(d *Definition) { d.Idempotency.Mode = "unknown" }},
		{"retry owner", func(d *Definition) { d.Retry.Owner = "unknown" }},
		{"usage owner", func(d *Definition) { d.Usage.Owner = "unknown" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			tt.mutate(&definition)

			require.Error(t, definition.Validate())
		})
	}
}

func TestDefinitionValidateEnforcesAICapabilityRetryAndUsageConsistency(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{
			"AI capability retry requires propose risk",
			func(d *Definition) {
				d.Retry.Owner = RetryOwnerAICapability
				d.Usage.Owner = UsageOwnerAICapability
			},
		},
		{
			"AI capability retry requires AI capability usage",
			func(d *Definition) {
				d.Risk = RiskPropose
				d.Retry.Owner = RetryOwnerAICapability
			},
		},
		{
			"AI capability usage requires propose risk",
			func(d *Definition) {
				d.Usage.Owner = UsageOwnerAICapability
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			tt.mutate(&definition)

			require.Error(t, definition.Validate())
		})
	}
}
