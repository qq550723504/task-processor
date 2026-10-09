package temporal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
)

func TestV3ImageSetInputPreservesFrozenSetAndItsFingerprint(t *testing.T) {
	set := &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, ParametersDigest: "parameters", InputDigest: "approved-input"}
	input := ExecuteSlotV3ActivityInput{RunID: "run", Identity: imageagent.ExecutionIdentity{TenantID: "org", UserID: "actor"}, PlanRevision: 1, Slot: imageagent.Slot{ID: "detail", Recipe: &imageagent.ImageSlotRecipe{Prompt: "approved prompt"}}, Attempt: 1, ImageSet: set}
	execution := slotExecutionInputV3(input)
	require.Equal(t, set, execution.ImageSet)
	before := imageagent.SlotExecutionFingerprint(execution)
	set.ParametersDigest = "changed"
	require.Equal(t, before, imageagent.SlotExecutionFingerprint(execution), "caller mutation must not change the captured set")
	input.Slot.Recipe.Prompt = "changed prompt"
	require.Equal(t, before, imageagent.SlotExecutionFingerprint(execution), "caller mutation must not change the captured recipe")
	changed := slotExecutionInputV3(input)
	require.NotEqual(t, before, imageagent.SlotExecutionFingerprint(changed))
	for _, payload := range []any{SlotWorkflowV3Input{}, ExecuteSlotV3ActivityInput{}, EffectRecoveryWorkflowInput{}} {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "ImageSet", "historical absent fields must remain absent")
	}
}

func TestSetWorkflowOffersKnownPartialResultsOnlyAfterEveryEffectCloses(t *testing.T) {
	_, repo, input := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	plan := current.Plan
	second := plan.Slots[0]
	second.ID, second.IdempotencyKey = "second", "second"
	second.Recipe = imageagent.CloneImageSlotRecipe(second.Recipe)
	second.Recipe.Placement.Order = 2
	plan.Slots = append(plan.Slots, second)
	plan.Set.MaxPoints = 24
	plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	hash := strings.Repeat("a", 64)
	proof := &imageagent.ImageGenerationProof{IntentID: hash, Fingerprint: hash, SettlementProofDigest: hash, Points: 12}
	results := []SlotWorkflowResult{
		{Execution: imageagent.SlotExecutionResult{SlotID: plan.Slots[0].ID, Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "generated", SourceAssetID: "source-1", Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-1/1/overview/1/0-" + hash + ".png", SHA256: hash}, GenerationProof: proof}}}, Status: imageagent.SlotStatusAccepted, Closure: &imageagent.ImageSlotClosure{Kind: "settled", IntentID: hash, Fingerprint: hash, SettlementProofDigest: hash, Points: 12}},
		{Execution: imageagent.SlotExecutionResult{SlotID: second.ID}, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.BudgetElapsedCode, Closure: &imageagent.ImageSlotClosure{Kind: "not_dispatched"}},
	}
	wire := workflowActivityWire{useV3Slot: true, useV3Approval: true}
	projection := summarizeResultsForWire(plan, results, wire)
	require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, projection.Status)
	require.Nil(t, projection.Block)
	digest, err := resultDigestForWire(plan, results, wire)
	require.NoError(t, err)
	require.NotEmpty(t, digest)
	acceptedClosure := results[0].Closure
	results[0].Closure = nil
	results[0].EffectPhase = imageagent.SlotEffectV3PublicationComplete
	require.Equal(t, imageagent.RunStatusBlocked, summarizeResultsForWire(plan, results, wire).Status, "materialized output without original settlement evidence must stay blocked")
	results[0].Closure = acceptedClosure
	results[1].Closure = nil
	results[1].Execution.Attempt = 1
	results[1].ErrorCode = imageagent.SlotProviderOutcomeUnknownCode
	results[1].EffectPhase = imageagent.SlotEffectV3ProviderUnknown
	projection = summarizeResultsForWire(plan, results, wire)
	require.Equal(t, imageagent.RunStatusBlocked, projection.Status)
	_, err = resultDigestForWire(plan, results, wire)
	require.Error(t, err)
}

type setPlanRepository struct {
	imageagent.Repository
	projection imageagent.RunProjection
}

func (r setPlanRepository) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return r.projection, nil
}

func TestV3ImageSetExecutionRejectsAChangedOrOmittedPersistedRecipe(t *testing.T) {
	activities, _, activityInput := imageSetPersistenceFixture(t)
	input := slotExecutionInputV3(activityInput)
	set := activityInput.ImageSet
	require.NoError(t, activities.validatePersistedImageSetExecution(context.Background(), input))
	input.ImageSet = nil
	require.ErrorIs(t, activities.validatePersistedImageSetExecution(context.Background(), input), imageagent.ErrRevisionConflict)
	input.ImageSet = imageagent.CloneImageSetPlan(set)
	input.Slot.Recipe = &imageagent.ImageSlotRecipe{Prompt: "changed prompt"}
	require.ErrorIs(t, activities.validatePersistedImageSetExecution(context.Background(), input), imageagent.ErrRevisionConflict)
}

func TestSetRegenerationCannotUseTheOriginalRunsRetryCommand(t *testing.T) {
	input := WorkflowInput{Plan: imageagent.Plan{Revision: 1, Set: &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema}, Slots: []imageagent.Slot{{ID: "detail"}}}}
	results := []SlotWorkflowResult{{Status: imageagent.SlotStatusBlocked}}
	projection := WorkflowResult{Status: imageagent.RunStatusBlocked, Block: &imageagent.Block{Code: imageagent.SlotProviderNotDispatchedCode, SlotID: "detail"}}
	state := workflowUpdateState{input: &input, results: &results, projection: &projection}
	err := state.validateRetrySlotBusiness(RetrySlotSignal{PlanRevision: 1, SlotID: "detail"})
	require.Error(t, err, "a new set generation must obtain a fresh plan confirmation and admission")
}

func TestSetRecoveryCompletionCannotDiscardSettlementEvidence(t *testing.T) {
	input := WorkflowInput{RunID: "run", Plan: imageagent.Plan{Revision: 1, Set: &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema}, Slots: []imageagent.Slot{{ID: "detail", Recipe: &imageagent.ImageSlotRecipe{Quote: imageagent.ImageGenerationQuote{Points: 12}}}}}}
	results := []SlotWorkflowResult{{Execution: imageagent.SlotExecutionResult{SlotID: "detail", Attempt: 1}, Status: imageagent.SlotStatusBlocked}}
	projection := WorkflowResult{Status: imageagent.RunStatusBlocked, Slots: []imageagent.SlotProjection{{Slot: input.Plan.Slots[0], Attempt: 1}}, RecoverableEffects: []imageagent.RecoverableEffect{{SlotID: "detail", Attempt: 1, Code: imageagent.SlotProviderOutcomeUnknownCode}}}
	state := workflowUpdateState{input: &input, results: &results, projection: &projection}
	signal := EffectRecoveryCompletedSignal{RunID: "run", PlanRevision: 1, SlotID: "detail", Attempt: 1, Result: EffectRecoveryResult{Outcome: EffectRecoveryOutcomePublished, EffectPhase: imageagent.SlotEffectV3PublicationComplete, Published: imageagent.SlotEffectV3PublishedResult{SlotID: "detail", Attempt: 1, Candidates: []imageagent.SlotEffectV3AssetCandidate{{AssetID: "candidate", SourceAssetID: "source", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-1/1/detail/1/0-" + v3SHA256 + ".png", SHA256: v3SHA256}, Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}}}}}}
	require.False(t, state.applyEffectRecoveryCompleted(signal), "a set recovery signal needs the original generation and settlement references")
	signal.Result.Closure = &imageagent.ImageSlotClosure{Kind: "settled", IntentID: v3SHA256, Fingerprint: v3SHA256, SettlementProofDigest: v3SHA256, Points: 12}
	signal.Result.GenerationProof = &imageagent.ImageGenerationProof{IntentID: v3SHA256, Fingerprint: v3SHA256, SettlementProofDigest: v3SHA256, Points: 13}
	require.False(t, state.applyEffectRecoveryCompleted(signal), "recovery must preserve the exact original point quote")
	signal.Result.GenerationProof.Points = 12
	require.True(t, state.applyEffectRecoveryCompleted(signal))
	require.Equal(t, signal.Result.Closure, projection.Slots[0].Closure)
	require.Equal(t, signal.Result.GenerationProof, projection.Slots[0].Candidates[0].GenerationProof)
}
