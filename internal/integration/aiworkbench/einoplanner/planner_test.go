package einoplanner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
	"task-processor/internal/aiworkbench"
)

func testProfile() aicapability.ModelProfile {
	return aicapability.ModelProfile{
		ClientName: "planning", ProviderID: "synthetic", AdapterKind: "openai-compatible", ModelID: "model",
		EndpointIdentityDigest: "digest", CredentialVersion: "v1", RoutingPolicyVersion: "route-v1",
		AdapterPolicyVersion: "adapter-v1", PromptVersion: "prompt-v1", OutputSchemaVersion: "schema-v1",
		UsageMappingVersion: "usage-v1", CostPricingVersion: "cost-v1",
		PointTariff: aicapability.ModelPointTariff{PriceVersion: "points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 1000000},
		Currency:    "USD", InputMicrosPerMillion: 1000000, OutputMicrosPerMillion: 1000000,
		MaximumPromptTokens: 8192, MaximumCompletionTokens: 64, MaximumInputBytes: 16 << 10,
		MaximumOutputBytes: 16 << 10, DeadlineBound: 5 * time.Second,
	}
}

func TestPrepareBindsServerScopeAndExactHistory(t *testing.T) {
	input := Request{Scope: aiworkbench.Scope{OrganizationID: "org-1", ActorID: "actor-1"}, MemberID: "member-1", InvocationID: "inv-1",
		OperationID: "operation-1", TargetPlatform: "shein", Profile: testProfile(),
		History: []aiworkbench.Message{{ID: "message-1", Sequence: 1, Author: aiworkbench.AuthorUser, Content: "please optimize the title"}},
	}
	prepared, err := Prepare(input)
	require.NoError(t, err)
	require.Equal(t, aicapability.OperationAIWorkbenchChatPlan, prepared.Identity.Operation)
	require.Contains(t, prepared.Identity.Prompt, `"operation_id":"operation-1"`)
	require.Contains(t, prepared.Identity.Prompt, `"target_platform":"shein"`)
	require.Equal(t, "", prepared.Identity.AgentRunID)
	require.NotEmpty(t, prepared.Quote.InputHash)

	input.History[0].Content = "different message"
	changed, err := Prepare(input)
	require.NoError(t, err)
	require.NotEqual(t, prepared.Quote.InputHash, changed.Quote.InputHash)
}

func TestStrictPlanningDecisionRejectsModelSelectedExecutionFacts(t *testing.T) {
	for _, raw := range []string{
		`{"mode":"READY","assistant_text":"Okay","goal_summary":"Title","operation_id":"other"}`,
		`{"mode":"READY","assistant_text":"Okay","goal_summary":""}`,
		`{"mode":"CLARIFY","assistant_text":"Question?","goal_summary":"hidden intent"}`,
		`{"mode":"READY","assistant_text":"Okay","goal_summary":"Title","mode":"CLARIFY"}`,
		`{"mode":"EXECUTE","assistant_text":"Okay","goal_summary":"Title"}`,
	} {
		_, err := ParseDecision(raw)
		require.Error(t, err, raw)
	}
	decision, err := ParseDecision(`{"mode":"READY","assistant_text":"I can suggest a title.","goal_summary":"Improve the saved product title"}`)
	require.NoError(t, err)
	require.Equal(t, aiworkbench.PlanReady, decision.Mode)
}
