package imageagent

import (
	"reflect"

	"task-processor/internal/agentconfig"
)

// ImageSetResultDigest is a separate contract from ResultDigestV3. It binds
// known terminal failures and unstarted work without declaring them accepted.
// Financial references must still be resolved by their owners on selection.
func ImageSetResultDigest(plan Plan, slots []SlotProjection, recoverable []RecoverableEffect) (string, error) {
	if plan.Set == nil || ValidateSubmittedPlan(plan) != nil || len(slots) != len(plan.Slots) || len(recoverable) != 0 {
		return "", ErrRevisionConflict
	}
	accepted := 0
	seen := map[string]bool{}
	for index, declared := range plan.Slots {
		actual := slots[index]
		definition := actual.Slot
		definition.Status = declared.Status
		if !reflect.DeepEqual(definition, declared) || actual.Closure == nil {
			return "", ErrRevisionConflict
		}
		closure := actual.Closure
		switch closure.Kind {
		case "settled":
			if !agentconfig.ImageDigest(closure.IntentID) || !agentconfig.ImageDigest(closure.Fingerprint) || !agentconfig.ImageDigest(closure.SettlementProofDigest) || closure.Points != declared.Recipe.Quote.Points || actual.Attempt <= 0 {
				return "", ErrRevisionConflict
			}
		case "no_generation":
			if !agentconfig.ImageDigest(closure.IntentID) || !agentconfig.ImageDigest(closure.Fingerprint) || !agentconfig.ImageDigest(closure.SettlementProofDigest) || closure.Points != 0 || actual.Attempt <= 0 {
				return "", ErrRevisionConflict
			}
		case "not_dispatched":
			if closure.IntentID != "" || closure.Fingerprint != "" || closure.SettlementProofDigest != "" || closure.Points != 0 || actual.Attempt != 0 {
				return "", ErrRevisionConflict
			}
		default:
			return "", ErrRevisionConflict
		}
		switch actual.Slot.Status {
		case SlotStatusAccepted:
			if closure.Kind != "settled" || actual.ErrorCode != "" || len(actual.Candidates) != 1 {
				return "", ErrRevisionConflict
			}
			candidate := actual.Candidates[0]
			proof := candidate.GenerationProof
			if ValidateProvenanceAssetID(candidate.AssetID) != nil || seen[candidate.AssetID] || candidate.SourceAssetID != declared.SourceAssetIDs[0] || candidate.Width <= 0 || candidate.Height <= 0 || len(candidate.Operations) != 1 || candidate.Operations[0] != "render_source_edit" || proof == nil || proof.IntentID != closure.IntentID || proof.Fingerprint != closure.Fingerprint || proof.SettlementProofDigest != closure.SettlementProofDigest || proof.Points != closure.Points {
				return "", ErrRevisionConflict
			}
			if _, err := NormalizeDurableAssetIdentity(candidate.DurableAsset); err != nil {
				return "", err
			}
			seen[candidate.AssetID] = true
			accepted++
		case SlotStatusBlocked, SlotStatusRejected:
			if !canonicalImageValue(actual.ErrorCode) || len(actual.Candidates) != 0 {
				return "", ErrRevisionConflict
			}
		default:
			return "", ErrRevisionConflict
		}
	}
	if accepted == 0 {
		return "", ErrCommandBlocked
	}
	// SlotProjection's explicit JSON preserves durable objects, observations,
	// original settlement references and each recipe in the ordered result.
	return resultDigestSHA256(struct {
		Schema string
		Plan   Plan
		Slots  []SlotProjection
	}{"product-image-set-result-v1", plan, slots})
}
