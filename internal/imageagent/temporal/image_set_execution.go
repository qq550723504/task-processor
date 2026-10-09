package temporal

import (
	"context"
	"reflect"

	"task-processor/internal/imageagent"
)

func recoverySlotExecutionInput(input EffectRecoveryWorkflowInput) imageagent.SlotExecutionInput {
	return slotExecutionInputV3(ExecuteSlotV3ActivityInput{RunID: input.RunID, Identity: input.Identity, ImageSet: imageagent.CloneImageSetPlan(input.ImageSet), TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext), PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt, IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt), AssetCatalog: input.AssetCatalog})
}

// Deserialized activity inputs consume the immutable plan held by ImageDB.
// An omitted set or a changed recipe cannot downgrade an approved set run to
// the earlier single-image execution contract.
func (a *Activities) validatePersistedImageSetExecution(ctx context.Context, input imageagent.SlotExecutionInput) error {
	projection, err := a.repository.GetProjection(ctx, imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID})
	if err != nil {
		return err
	}
	if projection.Plan.Set == nil && input.ImageSet == nil && input.Slot.Recipe == nil {
		return nil
	}
	if projection.Plan.Set == nil || input.ImageSet == nil || !reflect.DeepEqual(projection.Plan.Set, input.ImageSet) || projection.Plan.Revision != input.PlanRevision || projection.Run.ActivePlanRevision != input.PlanRevision || projection.Run.TargetPlatform != input.TargetPlatform || input.ImagePolicyContext != nil {
		return imageagent.ErrRevisionConflict
	}
	for _, declared := range projection.Plan.Slots {
		if declared.ID == input.Slot.ID {
			declared.Status = input.Slot.Status
			if reflect.DeepEqual(declared, input.Slot) {
				return nil
			}
			break
		}
	}
	return imageagent.ErrRevisionConflict
}
