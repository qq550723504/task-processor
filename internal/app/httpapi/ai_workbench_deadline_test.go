package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
	"task-processor/internal/aiworkbench"
	governed "task-processor/internal/integration/aicapability/einomodel"
	"task-processor/internal/integration/aiworkbench/einoplanner"
)

type delayedWorkbenchGenerator struct {
	deadline time.Time
	called   bool
	sentLate bool
}

func (g *delayedWorkbenchGenerator) Generate(ctx context.Context, _ aicapability.TextInputIdentity, _ aicapability.TextQuote, _ func(string) error) (governed.TextOutput, error) {
	g.called = true
	if remaining := time.Until(g.deadline.Add(10 * time.Millisecond)); remaining > 0 {
		time.Sleep(remaining)
	}
	g.sentLate = ctx.Err() == nil
	return governed.TextOutput{}, governed.ErrNotDispatched
}

func TestWorkbenchPlannerUsesFrozenDeadlineAtModelHandoff(t *testing.T) {
	profile := aicapability.ModelProfile{ClientName: "test", ProviderID: "test", AdapterKind: "openai-compatible", ModelID: "test",
		EndpointIdentityDigest: "digest", CredentialVersion: "v1", RoutingPolicyVersion: "v1", AdapterPolicyVersion: "v1",
		PromptVersion: "ai-workbench-chat-plan-v1", OutputSchemaVersion: "ai-workbench-plan-decision-v1", UsageMappingVersion: "v1", CostPricingVersion: "v1",
		PointTariff: aicapability.ModelPointTariff{PriceVersion: "v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 1000000},
		Currency:    "USD", InputMicrosPerMillion: 1, OutputMicrosPerMillion: 1, MaximumPromptTokens: 4096, MaximumCompletionTokens: 64,
		MaximumInputBytes: 16 << 10, MaximumOutputBytes: 16 << 10, DeadlineBound: time.Second}
	require.NoError(t, profile.Validate())
	scope := aiworkbench.Scope{OrganizationID: "org", ActorID: "actor"}
	history := []aiworkbench.Message{{ID: "message-1", Sequence: 1, Author: aiworkbench.AuthorUser, Content: "improve title"}}
	request := einoplanner.Request{Scope: scope, MemberID: "member", InvocationID: "invocation", OperationID: "operation",
		TargetPlatform: "shein", History: history, Profile: profile}
	prepared, err := einoplanner.Prepare(request)
	require.NoError(t, err)
	raw, err := json.Marshal(profile)
	require.NoError(t, err)
	deadline := time.Now().Add(2 * time.Second)
	command := aiworkbench.PlanningCommand{Scope: scope, MemberID: request.MemberID, PlannerInvocationID: request.InvocationID,
		WorkScope: aiworkbench.WorkScope{OperationID: request.OperationID, TargetPlatform: request.TargetPlatform},
		InputHash: prepared.Quote.InputHash, ModelProfile: raw, Deadline: deadline}
	generator := &delayedWorkbenchGenerator{deadline: deadline}
	planner := workbenchPlanner{model: einoplanner.Planner{Text: generator}}
	_, err = planner.Decide(context.Background(), command, history)
	require.ErrorIs(t, err, governed.ErrNotDispatched)
	require.True(t, generator.called, "the model handoff must be reached before the frozen deadline")
	require.False(t, generator.sentLate, "an expired planning command cannot send after SDK or admission delay")
}
