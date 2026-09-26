package temporal

import (
	"context"
	"errors"
	"reflect"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
)

// Called only after live identity and immutable catalog checks. Financial
// finalization intentionally has no reference to this output capability.
func (a *Activities) recoverSucceededGenerationOutput(ctx context.Context, input imageagent.SlotExecutionInput, reservation imageagent.SlotEffectV3Reservation, effect imageagent.SlotEffectV3Attempt) (imageagent.SlotEffectV3Attempt, error) {
	if a.generationOutputRecovery == nil || input.OrganizationIdentity.ScopeProtocol != imageagent.OrganizationScopeProtocol {
		return effect, nil
	}
	// Fence input identity before even a safe GET or immutable bundle write.
	// The store repeats this check under its owner transaction.
	if effect.Identity != reservation.Identity || effect.IdempotencyKey != reservation.IdempotencyKey || effect.InputFingerprint != reservation.InputFingerprint ||
		!reflect.DeepEqual(effect.Quote, reservation.Quote) || !reflect.DeepEqual(effect.Policy, reservation.Policy) {
		return effect, imageagent.ErrRevisionConflict
	}
	switch effect.Phase {
	case imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotEffectV3StagingUnknown:
	default:
		return effect, nil
	}
	facts, ok := a.repository.(imageagent.GenerationFactRepository)
	if !ok {
		return effect, imageagent.ErrValidation
	}
	fact, err := facts.ReadGenerationFact(ctx, reservation.Identity)
	if errors.Is(err, imageagent.ErrRunNotFound) {
		return effect, nil
	}
	if err != nil {
		return effect, err
	}
	if fact.Validate() != nil || fact.Intent.Identity != reservation.Identity || fact.Intent.MemberID != input.OrganizationIdentity.MemberID || fact.Intent.CatalogHash != input.AssetCatalog.Manifest.Hash {
		return effect, imageagent.ErrRevisionConflict
	}
	if fact.State != imageagent.GenerationSucceeded {
		return effect, nil
	}
	restorer, ok := a.slotEffectsV3.(imageagent.GenerationStagingRecoveryRepository)
	if !ok {
		return effect, imageagent.ErrValidation
	}
	prepared, err := a.artifactStore.RecoverSlotArtifacts(ctx, reservation.Identity, effect.StagingManifest)
	if err != nil {
		if !errors.Is(err, objectstore.ErrArtifactUnavailable) {
			return effect, imageagent.ErrValidation
		}
		if fact.Success.ResultURL == "" {
			return effect, blockedSlotEffectV3Error(slotProviderOutcomeUnknownCode)
		}
		generated, getErr := a.generationOutputRecovery(ctx, input, fact)
		if getErr != nil {
			return effect, imageagent.ErrValidation
		}
		prepared, err = prepareGeneratedSlotArtifacts(input, generated, a.artifactStore)
		if err != nil {
			return effect, imageagent.ErrValidation
		}
		if _, err = expectedFinalManifestV3(input, prepared.Manifest); err != nil {
			return effect, imageagent.ErrValidation
		}
		if err = a.preserveSlotRecoveryBundle(ctx, reservation.Identity, prepared); err != nil {
			// Concurrent safe GETs must converge on the first immutable bundle.
			if !errors.Is(err, objectstore.ErrObjectConflict) {
				return effect, imageagent.ErrValidation
			}
			prepared, err = a.artifactStore.RecoverSlotArtifacts(ctx, reservation.Identity, effect.StagingManifest)
			if err != nil {
				return effect, imageagent.ErrValidation
			}
		}
	}
	if _, err = expectedFinalManifestV3(input, prepared.Manifest); err != nil {
		return effect, imageagent.ErrValidation
	}
	return restorer.RecoverGenerationStaging(ctx, reservation, fact.Intent, fact.TerminalProofDigest(), prepared.Manifest)
}
