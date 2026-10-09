package imageagent

import (
	"time"

	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
)

func CloneImageAdmission(receipt *agentconfig.ImageRunAdmissionReceipt) *agentconfig.ImageRunAdmissionReceipt {
	if receipt == nil {
		return nil
	}
	clone := *receipt
	return &clone
}

// The configuration owner is the authority for admission. ImageDB keeps its
// exact immutable receipt so a replay cannot extend the admitted budget/time.
func ValidateImageSetAdmission(run Run, plan Plan) error {
	r := run.ImageAdmission
	if plan.Set == nil || r == nil || run.ScopeProtocol != OrganizationScopeProtocol || run.ImagePolicyContext != (ImagePolicyContext{}) || run.TargetPlatform != plan.Set.Target.Platform || !agentconfig.UUID(r.ID) || !agentconfig.ImageDigest(r.Digest) || r.AdmittedAt.IsZero() || !run.StartedAt.Equal(r.AdmittedAt) || r.Deadline.Sub(r.AdmittedAt) != time.Duration(r.Command.Limits.ElapsedSeconds)*time.Second {
		return ErrRevisionConflict
	}
	c := r.Command
	copyReceipt := *r
	copyReceipt.Digest = ""
	if generationHash(copyReceipt) != r.Digest {
		return ErrRevisionConflict
	}
	set := plan.Set
	if c.Scope != (agent.Scope{OrganizationID: run.TenantID, ActorID: run.UserID}) || c.MemberID != run.MemberID || c.RunID != run.ID || !agentconfig.UUID(c.ConfirmActionID) || c.Snapshot != set.Configuration || c.SourceDigest != ImageSetSourceDigest(set.Source, plan) || c.InputDigest != set.InputDigest || c.QuoteDigest != set.QuoteDigest || !c.Limits.Valid() || c.Limits.Images != len(plan.Slots) || c.Limits.Points != set.MaxPoints || run.Budget != ImageSetBudget(c.Limits) {
		return ErrRevisionConflict
	}
	// Slot status is execution state, not a change of the admitted plan.
	copy := plan
	copy.Slots = append([]Slot(nil), plan.Slots...)
	for i := range copy.Slots {
		copy.Slots[i].Status = SlotStatusPending
	}
	digest, err := ImageSetPlanDigest(copy)
	if err != nil || digest != c.PlanDigest {
		return ErrRevisionConflict
	}
	return nil
}

func ImageSetBudget(limits agentconfig.ImageRunLimits) Budget {
	return Budget{MaxImages: limits.Images, MaxModelCalls: limits.Images, MaxElapsed: time.Duration(limits.ElapsedSeconds) * time.Second, EnabledLimits: BudgetLimitImages | BudgetLimitModelCalls | BudgetLimitRepairAttemptsPerSlot | BudgetLimitElapsed}
}

func ImageSetSourceDigest(source ImageSourceBinding, plan Plan) string {
	byID := make(map[string]ImageSourceObservation)
	for _, slot := range plan.Slots {
		if slot.Recipe == nil {
			return ""
		}
		for _, ref := range slot.Recipe.References {
			if old, exists := byID[ref.AssetID]; exists && old != ref {
				return ""
			}
			byID[ref.AssetID] = ref
		}
	}
	ordered := make([]ImageSourceObservation, 0, len(plan.SourceAssetIDs))
	for _, id := range plan.SourceAssetIDs {
		ref, ok := byID[id]
		if !ok {
			return ""
		}
		ordered = append(ordered, ref)
	}
	return ImageSourceObservationsDigest(source, ordered)
}

func ImageSourceObservationsDigest(source ImageSourceBinding, observations []ImageSourceObservation) string {
	return generationHash(struct {
		Source     ImageSourceBinding
		References []ImageSourceObservation
	}{source, observations})
}
