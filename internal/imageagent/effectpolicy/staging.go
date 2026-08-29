package effectpolicy

import "task-processor/internal/imageagent"

// PrepareStaging decides whether a provider-claimed attempt can record a
// normalized staging manifest. It never mutates caller-owned state.
func PrepareStaging(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (EffectDecision, error) {
	normalized, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return EffectDecision{}, err
	}
	fingerprint, err := imageagent.StagingManifestFingerprint(normalized)
	if err != nil {
		return EffectDecision{}, err
	}
	if err := validateProviderReservation(reservation); err != nil {
		return EffectDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := validateProviderAttemptReservation(attempt, reservation); err != nil {
		return EffectDecision{}, err
	}
	decision := EffectDecision{Attempt: attempt}
	if attempt.Phase == imageagent.SlotEffectV3StagingPrepared {
		if attempt.StagingManifestFingerprint == fingerprint {
			return decision, nil
		}
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	if attempt.Phase != imageagent.SlotEffectV3ProviderClaimed {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	decision.Attempt.Phase = imageagent.SlotEffectV3StagingPrepared
	decision.Attempt.StagingManifest = imageagent.StagingManifest{Assets: cloneStagedAssetRefs(normalized.Assets)}
	decision.Attempt.StagingManifestFingerprint = fingerprint
	decision.Changed = true
	return decision, nil
}

// CommitStaged decides whether a prepared staging manifest can be marked as
// durably staged. It never mutates caller-owned state.
func CommitStaged(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, manifestFingerprint string) (EffectDecision, error) {
	if err := validateProviderReservation(reservation); err != nil {
		return EffectDecision{}, err
	}
	attempt := cloneSlotEffectV3Attempt(current)
	if err := validateProviderAttemptReservation(attempt, reservation); err != nil {
		return EffectDecision{}, err
	}
	if attempt.StagingManifestFingerprint != manifestFingerprint {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	decision := EffectDecision{Attempt: attempt}
	if attempt.Phase == imageagent.SlotEffectV3ArtifactStaged {
		return decision, nil
	}
	if attempt.Phase != imageagent.SlotEffectV3StagingPrepared {
		return EffectDecision{}, imageagent.ErrRevisionConflict
	}
	decision.Attempt.Phase = imageagent.SlotEffectV3ArtifactStaged
	decision.Changed = true
	return decision, nil
}
