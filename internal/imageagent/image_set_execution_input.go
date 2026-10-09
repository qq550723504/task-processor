package imageagent

import "fmt"

func SlotAttemptKey(planRevision int64, slot Slot, attempt int) string {
	return fmt.Sprintf("%s:plan:%d:attempt:%d", slot.IdempotencyKey, planRevision, attempt)
}

func ImageSetSlotExecutionInput(projection RunProjection, slotID string, attempt int) (SlotExecutionInput, error) {
	if attempt <= 0 || ValidateImageSetAdmission(projection.Run, projection.Plan) != nil {
		return SlotExecutionInput{}, ErrRevisionConflict
	}
	for _, declared := range projection.Plan.Slots {
		if declared.ID != slotID {
			continue
		}
		declared.Recipe = CloneImageSlotRecipe(declared.Recipe)
		declared.SourceAssetIDs = append([]string(nil), declared.SourceAssetIDs...)
		return SlotExecutionInput{RunID: projection.Run.ID, TenantID: projection.Run.TenantID, UserID: projection.Run.UserID, TargetPlatform: projection.Run.TargetPlatform, PlanRevision: projection.Plan.Revision, Slot: declared, Attempt: attempt, IdempotencyKey: SlotAttemptKey(projection.Plan.Revision, declared, attempt), AssetCatalog: projection.AssetCatalog, ProductContext: projection.AssetCatalog.ProductContext, ImageSet: CloneImageSetPlan(projection.Plan.Set)}, nil
	}
	return SlotExecutionInput{}, ErrRevisionConflict
}
