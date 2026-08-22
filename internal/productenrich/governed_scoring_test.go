package productenrich

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/aicapability"
)

func TestScoreCacheIdentityPartitionsGovernedExecutions(t *testing.T) {
	base := ScoreCacheIdentity{
		Version: 1, TenantID: "tenant-a",
		Capability: aicapability.CapabilityProductEnrichText,
		Operation:  aicapability.OperationProductEnrichTextQualityScore,
		RouteMode:  aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		ProviderID: "openai", ModelID: "gpt-4.1-mini", RoutingKey: "fast",
		PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		PromptKey: "productenrich.llm_scorer.text_scoring", PromptVersion: "v1", PromptScope: "product_enrich",
		BaseScore: "80", InputHash: "input-a",
	}
	variants := []struct {
		name   string
		mutate func(*ScoreCacheIdentity)
	}{
		{"schema version", func(v *ScoreCacheIdentity) { v.Version = 2 }},
		{"tenant", func(v *ScoreCacheIdentity) { v.TenantID = "tenant-b" }},
		{"capability", func(v *ScoreCacheIdentity) { v.Capability = aicapability.CapabilityProductEnrichVision }},
		{"operation", func(v *ScoreCacheIdentity) { v.Operation = aicapability.OperationProductEnrichVisionQualityScore }},
		{"route mode", func(v *ScoreCacheIdentity) { v.RouteMode = aicapability.RoutingModeLegacy }},
		{"route outcome", func(v *ScoreCacheIdentity) { v.RouteOutcome = aicapability.RouteOutcomeLegacy }},
		{"provider", func(v *ScoreCacheIdentity) { v.ProviderID = "gemini" }},
		{"model", func(v *ScoreCacheIdentity) { v.ModelID = "gemini-2.5-flash" }},
		{"routing key", func(v *ScoreCacheIdentity) { v.RoutingKey = "quality" }},
		{"policy version", func(v *ScoreCacheIdentity) { v.PolicyVersion = "policy-v2" }},
		{"configuration version", func(v *ScoreCacheIdentity) { v.ConfigurationVersion = "config-v2" }},
		{"prompt key", func(v *ScoreCacheIdentity) { v.PromptKey = "other.prompt" }},
		{"prompt version", func(v *ScoreCacheIdentity) { v.PromptVersion = "prompt-v2" }},
		{"prompt scope", func(v *ScoreCacheIdentity) { v.PromptScope = "other_scope" }},
		{"base score", func(v *ScoreCacheIdentity) { v.BaseScore = "81" }},
		{"input hash", func(v *ScoreCacheIdentity) { v.InputHash = "different-input" }},
	}
	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			variant := base
			tc.mutate(&variant)
			require.NotEqual(t, base.Key(), variant.Key())
		})
	}
}

func TestScoreCacheIdentityRequiresVersionAndNormalizesStrings(t *testing.T) {
	identity := ScoreCacheIdentity{
		Version: 1, TenantID: "tenant-a",
		Capability: aicapability.CapabilityProductEnrichText,
		Operation:  aicapability.OperationProductEnrichTextQualityScore,
		RouteMode:  aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		ProviderID: "openai", ModelID: "gpt-4.1-mini", RoutingKey: "fast",
		PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		PromptKey: "prompt-key", PromptVersion: "prompt-v1", PromptScope: "product_enrich",
		BaseScore: "80", InputHash: "input-hash",
	}

	require.True(t, strings.HasPrefix(identity.Key(), "llm_score:governed:v1:"))
	spaced := identity
	spaced.TenantID = " tenant-a "
	spaced.PromptVersion = " prompt-v1 "
	require.Equal(t, identity.Key(), spaced.Key())
	identity.Version = 0
	require.Empty(t, identity.Key())
}
