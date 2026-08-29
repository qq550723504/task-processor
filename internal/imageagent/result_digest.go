package imageagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ResultDigestV2 is the frozen candidate-ID-only approval digest. Its JSON
// payload deliberately preserves the historical wire algorithm byte-for-byte.
func ResultDigestV2(plan Plan, slots []SlotProjection) (string, error) {
	if len(slots) != len(plan.Slots) {
		return "", ErrRevisionConflict
	}
	type digestSlot struct {
		SlotID            string   `json:"slot_id"`
		CandidateAssetIDs []string `json:"candidate_asset_ids"`
	}
	payload := make([]digestSlot, 0, len(plan.Slots))
	for index, slot := range plan.Slots {
		candidateIDs := make([]string, 0, len(slots[index].Candidates))
		for _, candidate := range slots[index].Candidates {
			id := strings.TrimSpace(candidate.AssetID)
			if id == "" {
				return "", ErrRevisionConflict
			}
			candidateIDs = append(candidateIDs, id)
		}
		payload = append(payload, digestSlot{SlotID: strings.TrimSpace(slot.ID), CandidateAssetIDs: candidateIDs})
	}
	return resultDigestSHA256(payload)
}

// ResultDigestV3 binds approval to the declared revision, ordered slot roles,
// candidate order, and durable final object identities.
func ResultDigestV3(plan Plan, slots []SlotProjection) (string, error) {
	if plan.Revision <= 0 || len(slots) != len(plan.Slots) {
		return "", ErrRevisionConflict
	}
	type digestCandidate struct {
		AssetID   string `json:"asset_id"`
		ObjectKey string `json:"object_key"`
		SHA256    string `json:"sha256"`
	}
	type digestSlot struct {
		SlotID     string            `json:"slot_id"`
		Role       SlotRole          `json:"role"`
		Candidates []digestCandidate `json:"candidates"`
	}
	payload := struct {
		PlanRevision int64        `json:"plan_revision"`
		Slots        []digestSlot `json:"slots"`
	}{PlanRevision: plan.Revision, Slots: make([]digestSlot, 0, len(plan.Slots))}
	seenCandidates := make(map[string]struct{})
	for index, declared := range plan.Slots {
		slot := slots[index]
		if strings.TrimSpace(declared.ID) == "" || slot.Slot.ID != declared.ID || slot.Slot.Role != declared.Role || slot.Slot.Status != SlotStatusAccepted || len(slot.Candidates) == 0 {
			return "", ErrRevisionConflict
		}
		entry := digestSlot{SlotID: declared.ID, Role: declared.Role, Candidates: make([]digestCandidate, 0, len(slot.Candidates))}
		for _, candidate := range slot.Candidates {
			assetID := strings.TrimSpace(candidate.AssetID)
			if assetID == "" {
				return "", ErrRevisionConflict
			}
			if _, duplicate := seenCandidates[assetID]; duplicate {
				return "", ErrRevisionConflict
			}
			identity, err := NormalizeDurableAssetIdentity(candidate.DurableAsset)
			if err != nil {
				return "", err
			}
			seenCandidates[assetID] = struct{}{}
			entry.Candidates = append(entry.Candidates, digestCandidate{AssetID: assetID, ObjectKey: identity.ObjectKey, SHA256: identity.SHA256})
		}
		payload.Slots = append(payload.Slots, entry)
	}
	return resultDigestSHA256(payload)
}

func resultDigestSHA256(payload any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
