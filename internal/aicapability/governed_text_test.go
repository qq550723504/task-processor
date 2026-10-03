package aicapability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testTextProfile() ModelProfile {
	return ModelProfile{
		ClientName: "title", ProviderID: "synthetic-openai", AdapterKind: "openai-compatible",
		ModelID: "model-a", EndpointIdentityDigest: "endpoint-v1", CredentialVersion: "credential-v1",
		RoutingPolicyVersion: "route-v1", AdapterPolicyVersion: "adapter-v1",
		PromptVersion: "prompt-v1", OutputSchemaVersion: "schema-v1",
		UsageMappingVersion: "usage-v1", CostPricingVersion: "cost-v1",
		PointTariff: ModelPointTariff{PriceVersion: "points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 2000000},
		Currency:    "CNY", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000,
		MaximumPromptTokens: 10000, MaximumCompletionTokens: 1000,
		MaximumInputBytes: 8192, MaximumOutputBytes: 16384, DeadlineBound: time.Minute,
	}
}

func TestModelProfileFreezesRouteCredentialAndPricing(t *testing.T) {
	profile := testTextProfile()
	require.NoError(t, profile.Validate())
	first, err := profile.Digest()
	require.NoError(t, err)
	for _, change := range []func(*ModelProfile){
		func(p *ModelProfile) { p.ProviderID = "other" },
		func(p *ModelProfile) { p.ModelID = "model-b" },
		func(p *ModelProfile) { p.CredentialVersion = "credential-v2" },
		func(p *ModelProfile) { p.CostPricingVersion = "cost-v2" },
		func(p *ModelProfile) { p.PromptVersion = "prompt-v2" },
		func(p *ModelProfile) { p.PointTariff.PriceVersion = "points-v2" },
	} {
		changed := profile
		change(&changed)
		digest, err := changed.Digest()
		require.NoError(t, err)
		require.NotEqual(t, first, digest)
	}
	invalid := profile
	invalid.CredentialVersion = ""
	require.Error(t, invalid.Validate())
}

func TestGovernedTextQuoteBindsCompleteInputAndReservesCeiling(t *testing.T) {
	profile := testTextProfile()
	input := TextInputIdentity{
		OrganizationID: "org-a", ActorID: "user-a", MemberID: "member-a",
		Operation: OperationAIWorkbenchChatPlan, InvocationID: "invocation-a",
		System: "system", Prompt: "prompt", Profile: profile,
	}
	quote, err := QuoteText(input)
	require.NoError(t, err)
	require.Greater(t, quote.MaximumTokens, int64(1000))
	require.Less(t, quote.MaximumTokens, int64(11000))
	cost, err := profile.CostFor(quote.MaximumTokens-profile.MaximumCompletionTokens, profile.MaximumCompletionTokens)
	require.NoError(t, err)
	require.Equal(t, cost, quote.MaximumCostMicros)
	require.Greater(t, quote.MaximumCostMicros, int64(0))
	require.NotEmpty(t, quote.InputHash)
	changed := input
	changed.Profile.CredentialVersion = "credential-v2"
	other, err := QuoteText(changed)
	require.NoError(t, err)
	require.NotEqual(t, quote.InputHash, other.InputHash)
	changed = input
	changed.Prompt = "new prompt"
	other, err = QuoteText(changed)
	require.NoError(t, err)
	require.NotEqual(t, quote.InputHash, other.InputHash)
	changed = input
	changed.Profile.MaximumInputBytes = 1
	_, err = QuoteText(changed)
	require.Error(t, err)
}
