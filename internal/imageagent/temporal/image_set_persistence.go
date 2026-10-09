package temporal

import (
	"context"
	"errors"
	"reflect"

	"task-processor/internal/imageagent"
)

// The additive set activity returns the same slot fact committed by the
// existing projection transaction. It does not create a second result owner.
func (a *Activities) PersistImageSetSlotResult(ctx context.Context, input PersistSlotResultV3ActivityInput) (imageagent.SlotProjection, error) {
	if err := a.PersistSlotResultV3(ctx, input); err != nil {
		return imageagent.SlotProjection{}, err
	}
	projection, err := a.repository.GetProjection(ctx, imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	if err != nil {
		return imageagent.SlotProjection{}, err
	}
	if projection.Plan.Set == nil || projection.Plan.Revision != input.PlanRevision {
		return imageagent.SlotProjection{}, imageagent.ErrRevisionConflict
	}
	for _, slot := range projection.Slots {
		if slot.Slot.ID == input.Result.Published.SlotID {
			return slot, nil
		}
	}
	return imageagent.SlotProjection{}, imageagent.ErrRevisionConflict
}

func (a *Activities) deriveImageSetSlotProjection(ctx context.Context, current imageagent.RunProjection, input PersistSlotResultV3ActivityInput, result imageagent.SlotProjection) (imageagent.SlotProjection, error) {
	if current.Run.ScopeProtocol != imageagent.OrganizationScopeProtocol || current.Plan.Set == nil || current.Plan.Revision != input.PlanRevision || current.Run.ActivePlanRevision != input.PlanRevision {
		return result, imageagent.ErrRevisionConflict
	}
	declared := findSlot(current.Plan, result.Slot.ID)
	if declared.ID == "" || input.AttemptKey != slotAttemptKey(input.PlanRevision, declared, input.Result.Published.Attempt) {
		return result, imageagent.ErrRevisionConflict
	}
	execution := slotExecutionInputV3(ExecuteSlotV3ActivityInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: input.PlanRevision, Slot: declared, Attempt: input.Result.Published.Attempt, IdempotencyKey: input.AttemptKey, TargetPlatform: current.Run.TargetPlatform, ImageSet: current.Plan.Set, AssetCatalog: current.AssetCatalog})
	reservation := slotEffectReservationV3(execution)
	effect, effectErr := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	if effectErr != nil && !errors.Is(effectErr, imageagent.ErrRunNotFound) {
		return result, effectErr
	}
	if effectErr == nil && (validatePersistedSlotEffectV3(effect) != nil || effect.InputFingerprint != reservation.InputFingerprint || effect.IdempotencyKey != reservation.IdempotencyKey) {
		return result, imageagent.ErrRevisionConflict
	}
	facts, ok := a.repository.(imageagent.GenerationFactRepository)
	if !ok {
		return result, imageagent.ErrValidation
	}
	fact, err := facts.ReadGenerationFact(ctx, reservation.Identity)
	if errors.Is(err, imageagent.ErrRunNotFound) {
		// Absence alone is not no-dispatch proof: an unknown activity may still
		// be finishing. Only an explicit original pre-dispatch outcome qualifies.
		if result.Slot.Status == imageagent.SlotStatusBlocked && input.Result.EffectPhase == imageagent.SlotEffectV3ProviderNotDispatched && terminalEffectPhaseForErrorCode(result.ErrorCode) == imageagent.SlotEffectV3ProviderNotDispatched && (errors.Is(effectErr, imageagent.ErrRunNotFound) || effect.Phase == imageagent.SlotEffectV3ProviderNotDispatched) {
			result.Attempt = 0
			result.Closure = &imageagent.ImageSlotClosure{Kind: "not_dispatched"}
			return result, nil
		}
		if result.Slot.Status == imageagent.SlotStatusAccepted {
			return result, imageagent.ErrRevisionConflict
		}
		return result, nil
	}
	if err != nil {
		return result, err
	}
	digest, err := imageagent.ImageSlotGenerationInputDigest(execution)
	if err != nil || fact.Validate() != nil || fact.Intent.Identity != reservation.Identity || fact.Intent.MemberID != input.Identity.MemberID || fact.Intent.CatalogHash != current.AssetCatalog.Manifest.Hash || fact.Intent.InputProtocol != imageagent.ImageSetSchema || fact.Intent.InputDigest != digest || fact.Intent.SourceDigest != imageagent.ImageSourceBundleDigest(declared.Recipe.References) || fact.Intent.PromptVersion != declared.Recipe.PromptVersion || !declared.Recipe.Quote.MatchesGenerationIntent(fact.Intent) {
		return result, imageagent.ErrRevisionConflict
	}
	closure, err := imageagent.ImageGenerationClosure(fact)
	if err != nil {
		if errors.Is(err, imageagent.ErrCommandBlocked) && result.Slot.Status != imageagent.SlotStatusAccepted {
			return result, nil
		}
		return result, err
	}
	if result.Slot.Status != imageagent.SlotStatusAccepted {
		invalidOutput := fact.State == imageagent.GenerationSucceeded && fact.Success.ResultUnavailable == "invalid_result"
		if fact.State == imageagent.GenerationSucceeded && !invalidOutput {
			invalidOutput = hasClosedInvalidImageSetOutput(current, execution, closure)
			if !invalidOutput && a.generationOutputRecovery != nil && effectErr == nil {
				switch effect.Phase {
				case imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotEffectV3StagingUnknown, imageagent.SlotEffectV3RecoveryBlocked:
					if err := a.validateOrganizationCatalog(ctx, input.Identity, input.RunID, current.AssetCatalog); err != nil {
						return result, err
					}
					_, outputErr := a.generationOutputRecovery(ctx, execution, fact)
					invalidOutput = errors.Is(outputErr, imageagent.ErrInvalidGeneratedOutput)
				}
			}
		}
		if fact.State == imageagent.GenerationNoEffect || invalidOutput {
			result.Closure = closure
			if fact.State == imageagent.GenerationNoEffect {
				result.ErrorCode = imageagent.SlotProviderNotDispatchedCode
			}
			if fact.State == imageagent.GenerationSucceeded {
				result.ErrorCode = imageagent.InvalidGeneratedOutputCode
			}
		}
		return result, nil
	}
	if fact.State != imageagent.GenerationSucceeded || effectErr != nil || effect.Phase != imageagent.SlotEffectV3PublicationComplete || !reflect.DeepEqual(effect.Published, input.Result.Published) || len(result.Candidates) != 1 {
		return result, imageagent.ErrRevisionConflict
	}
	candidate := &result.Candidates[0]
	if candidate.SourceAssetID != declared.SourceAssetIDs[0] || candidate.Width != 1024 || candidate.Height != 1024 || !reflect.DeepEqual(candidate.Operations, []string{"render_source_edit"}) {
		return result, imageagent.ErrRevisionConflict
	}
	result.Closure = closure
	candidate.GenerationProof = &imageagent.ImageGenerationProof{IntentID: closure.IntentID, Fingerprint: closure.Fingerprint, SettlementProofDigest: closure.SettlementProofDigest, Points: closure.Points}
	return result, nil
}

// The already-committed closure is the output decision. Its economic identity
// is always re-derived from the original settled generation fact.
func hasClosedInvalidImageSetOutput(current imageagent.RunProjection, input imageagent.SlotExecutionInput, closure *imageagent.ImageSlotClosure) bool {
	if current.Plan.Set == nil || current.Plan.Revision != input.PlanRevision || current.Run.ActivePlanRevision != input.PlanRevision {
		return false
	}
	for _, slot := range current.Slots {
		if slot.Slot.ID == input.Slot.ID && slot.Attempt == input.Attempt && slot.Slot.Status == imageagent.SlotStatusBlocked && slot.ErrorCode == imageagent.InvalidGeneratedOutputCode && reflect.DeepEqual(slot.Closure, closure) {
			return true
		}
	}
	return false
}
