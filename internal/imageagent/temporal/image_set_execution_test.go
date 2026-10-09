package temporal

import (
	"context"
	"encoding/json"
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

type setPlanRepository struct {
	imageagent.Repository
	projection imageagent.RunProjection
}

func (r setPlanRepository) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return r.projection, nil
}

func TestV3ImageSetExecutionRejectsAChangedOrOmittedPersistedRecipe(t *testing.T) {
	set := &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, InputDigest: "approved"}
	slot := imageagent.Slot{ID: "detail", Recipe: &imageagent.ImageSlotRecipe{Prompt: "approved prompt"}}
	projection := imageagent.RunProjection{Run: imageagent.Run{TargetPlatform: "product", ActivePlanRevision: 1}, Plan: imageagent.Plan{Revision: 1, Set: set, Slots: []imageagent.Slot{slot}}}
	activities := &Activities{repository: setPlanRepository{projection: projection}}
	input := imageagent.SlotExecutionInput{RunID: "run", TenantID: "org", UserID: "actor", TargetPlatform: "product", PlanRevision: 1, Slot: slot, ImageSet: imageagent.CloneImageSetPlan(set)}
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
