package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"

	"task-processor/internal/imageagent"
)

func (a *Activities) RecoverEffectV3(ctx context.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	if a.slotEffectsV3 == nil || a.stagedSlotExecutor == nil || a.artifactStore == nil || a.publicationOwner == nil {
		return EffectRecoveryResult{}, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	executionInput := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog,
		ProductContext: input.AssetCatalog.ProductContext,
	}
	reservation := slotEffectReservationV3(executionInput)
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	if err != nil {
		if errors.Is(err, imageagent.ErrRunNotFound) {
			return a.persistMissingEffectRecoveryBlockedV3(ctx, input, reservation)
		}
		if result, handled, blockErr := a.failClosedCorruptEffectRecovery(ctx, input, err); handled {
			return result, blockErr
		}
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	reservation.Policy = effect.Policy
	reservation.Quote = effect.Quote
	if effect.Phase == imageagent.SlotEffectV3RecoveryBlocked && strings.TrimSpace(input.ActionID) != "" {
		restorer, ok := a.slotEffectsV3.(imageagent.RecoveryBlockedSlotEffectV3Repository)
		if !ok {
			return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(imageagent.ErrRevisionConflict)
		}
		effect, err = restorer.RestoreRecoveryBlockedEffectV3(ctx, reservation)
		if err != nil {
			return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
		}
	}
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return EffectRecoveryResult{
			Outcome: EffectRecoveryOutcomePublished, Published: effect.Published,
			EffectPhase: effect.Phase,
		}, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, effect.Phase, slotProviderOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3StagingUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, effect.Phase, slotStagingOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3PublicationUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, effect.Phase, slotPublicationOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3ReviewRequired:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, effect.Phase, imageagent.SlotReviewRequiredCode), nil
	case imageagent.SlotEffectV3ReviewTransportRequired:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, effect.Phase, imageagent.SlotReviewTransportRequiredCode), nil
	case imageagent.SlotEffectV3ProviderNotDispatched:
		return a.blockEffectRecoveryV3(ctx, input, reservation)
	case imageagent.SlotEffectV3ProviderClaimed:
		if effect.Quote.Fingerprint != "" && effect.BudgetStatus == imageagent.SlotBudgetReleased {
			return a.blockEffectRecoveryV3(ctx, input, reservation)
		}
	case imageagent.SlotEffectV3RecoveryBlocked:
		return effectRecoveryBlockedResult(input), nil
	}
	budgetAuthorization := effect.Quote.Fingerprint != ""
	published, err := a.ExecuteSlotV3(ctx, ExecuteSlotV3ActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.PlanRevision,
		TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
		Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey:             executionInput.IdempotencyKey,
		AssetCatalog:               input.AssetCatalog,
		ExternalEffectFinalization: true,
		BudgetAuthorization:        budgetAuthorization,
		BudgetPolicy:               effect.Policy,
	})
	if err == nil {
		return EffectRecoveryResult{
			Outcome: EffectRecoveryOutcomePublished, Published: published,
			EffectPhase: imageagent.SlotEffectV3PublicationComplete,
		}, nil
	}
	if result, mapped := effectRecoveryResultFromError(err); mapped {
		return result, nil
	}
	return EffectRecoveryResult{}, err
}

func (a *Activities) PersistRecoveryBlockedEffectV3(ctx context.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	if a.slotEffectsV3 == nil {
		return EffectRecoveryResult{}, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	executionInput := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog,
		ProductContext: input.AssetCatalog.ProductContext,
	}
	reservation := slotEffectReservationV3(executionInput)
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	if err != nil {
		if errors.Is(err, imageagent.ErrRunNotFound) {
			return a.persistMissingEffectRecoveryBlockedV3(ctx, input, reservation)
		}
		if result, handled, blockErr := a.failClosedCorruptEffectRecovery(ctx, input, err); handled {
			return result, blockErr
		}
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	reservation.Policy = effect.Policy
	reservation.Quote = effect.Quote
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return EffectRecoveryResult{Outcome: EffectRecoveryOutcomePublished, Published: effect.Published, EffectPhase: effect.Phase}, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, effect.Phase, slotProviderOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3StagingUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, effect.Phase, slotStagingOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3PublicationUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, effect.Phase, slotPublicationOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3ReviewRequired:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, effect.Phase, imageagent.SlotReviewRequiredCode), nil
	case imageagent.SlotEffectV3ReviewTransportRequired:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, effect.Phase, imageagent.SlotReviewTransportRequiredCode), nil
	case imageagent.SlotEffectV3RecoveryBlocked:
		return effectRecoveryBlockedResult(input), nil
	}
	return a.blockEffectRecoveryV3(ctx, input, reservation)
}

func (a *Activities) StartEffectRecoveryV3(ctx context.Context, input EffectRecoveryWorkflowInput) error {
	if a.recoveryWorkflowStarter == nil {
		return fmt.Errorf("image agent effect recovery workflow starter is required")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	return a.recoveryWorkflowStarter(ctx, input)
}

func (a *Activities) ReconcileEffectRecoveryV3(ctx context.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	if a.slotEffectsV3 == nil {
		return EffectRecoveryResult{}, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	execution := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog, ProductContext: input.AssetCatalog.ProductContext,
	}
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, slotEffectReservationV3(execution).Identity)
	if err != nil {
		if result, handled, blockErr := a.failClosedCorruptEffectRecovery(ctx, input, err); handled {
			return result, blockErr
		}
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	result, err := effectRecoveryResultFromDurableEffect(effect)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	if err := a.reconcileEffectRecoveryProjection(ctx, input, effect, result); err != nil {
		return EffectRecoveryResult{}, err
	}
	return result, nil
}

func (a *Activities) failClosedCorruptEffectRecovery(ctx context.Context, input EffectRecoveryWorkflowInput, cause error) (EffectRecoveryResult, bool, error) {
	if !errors.Is(cause, imageagent.ErrCorruptPersistedEffect) {
		return EffectRecoveryResult{}, false, nil
	}
	corruptor, ok := a.slotEffectsV3.(imageagent.CorruptSlotEffectV3Repository)
	if !ok {
		return EffectRecoveryResult{}, true, persistedSlotEffectV3RepositoryError(cause)
	}
	identity := imageagent.SlotExternalEffectIdentity{
		RunScope:     imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID},
		PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
	}
	blocked, err := corruptor.BlockCorruptSlotEffectV3(ctx, identity)
	if err != nil {
		return EffectRecoveryResult{}, true, persistedSlotEffectV3RepositoryError(err)
	}
	return EffectRecoveryResult{
		Outcome:     EffectRecoveryOutcomeRecoveryBlocked,
		Published:   imageagent.SlotEffectV3PublishedResult{SlotID: strings.TrimSpace(input.Slot.ID), Attempt: input.Attempt},
		EffectPhase: blocked.Phase, BlockedCode: blocked.BlockedCode,
	}, true, nil
}

func effectRecoveryResultFromDurableEffect(effect imageagent.SlotEffectV3Attempt) (EffectRecoveryResult, error) {
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return EffectRecoveryResult{Outcome: EffectRecoveryOutcomePublished, Published: effect.Published, EffectPhase: effect.Phase}, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3StagingUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3PublicationUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3ReviewRequired:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3ReviewTransportRequired:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3RecoveryBlocked:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeRecoveryBlocked, effect.Phase, effect.BlockedCode), nil
	default:
		return EffectRecoveryResult{}, fmt.Errorf("effect recovery phase %q is not durable reconciliation evidence", effect.Phase)
	}
}

func (a *Activities) reconcileEffectRecoveryProjection(ctx context.Context, input EffectRecoveryWorkflowInput, effect imageagent.SlotEffectV3Attempt, result EffectRecoveryResult) error {
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("get image agent recovery parent projection: %w", err)
	}
	if current.Run.Status != imageagent.RunStatusBlocked || current.Run.ActivePlanRevision != input.PlanRevision || current.Plan.Revision != input.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	slotIndex := slotProjectionIndex(current.Slots, input.Slot.ID)
	if slotIndex < 0 || current.Slots[slotIndex].Attempt != input.Attempt {
		return imageagent.ErrRevisionConflict
	}
	owners, err := imageagent.NormalizeRecoverableEffects(current.RecoverableEffects)
	if err != nil {
		return err
	}
	ownerIndex := recoverableEffectIndex(owners, input.Slot.ID, input.Attempt)
	if ownerIndex < 0 {
		if effect.Phase == imageagent.SlotEffectV3PublicationComplete && recoveredSlotProjectionMatches(current.Slots[slotIndex], effect.Published) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	if effect.Phase != imageagent.SlotEffectV3PublicationComplete && recoveredBlockedProjectionMatches(current, slotIndex, ownerIndex, result.BlockedCode) {
		return nil
	}
	updated := current
	updated.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
	updated.RecoverableEffects = append([]imageagent.RecoverableEffect(nil), owners...)
	updated.Run.Block = cloneTemporalBlock(current.Run.Block)
	updated.Run.Version++
	if effect.Phase == imageagent.SlotEffectV3PublicationComplete {
		published, normalizeErr := imageagent.NormalizeSlotEffectV3PublishedResult(effect.Published)
		if normalizeErr != nil {
			return normalizeErr
		}
		candidates := make([]imageagent.AssetCandidate, 0, len(published.Candidates))
		for _, candidate := range published.Candidates {
			candidates = append(candidates, imageagent.AssetCandidate{
				AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
				Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
			})
		}
		updated.Slots[slotIndex] = imageagent.SlotProjection{
			Slot: updated.Slots[slotIndex].Slot, Attempt: published.Attempt, Candidates: candidates,
		}
		updated.Slots[slotIndex].Slot.Status = imageagent.SlotStatusAccepted
		updated.RecoverableEffects = append(updated.RecoverableEffects[:ownerIndex], updated.RecoverableEffects[ownerIndex+1:]...)
		if updated.Run.Block != nil && strings.TrimSpace(updated.Run.Block.SlotID) == strings.TrimSpace(input.Slot.ID) {
			updated.Run.Block = recoveryParentBlock(updated.RecoverableEffects)
		}
	} else {
		updated.RecoverableEffects[ownerIndex].Code = result.BlockedCode
		updated.Slots[slotIndex].ErrorCode = result.BlockedCode
		if updated.Run.Block != nil && strings.TrimSpace(updated.Run.Block.SlotID) == strings.TrimSpace(input.Slot.ID) {
			updated.Run.Block.Code = result.BlockedCode
			updated.Run.Block.Message = result.BlockedCode
		}
	}
	commitID, err := effectRecoveryReconciliationCommitID(input, effect.Phase)
	if err != nil {
		return err
	}
	candidateIDs := make([]string, 0, len(updated.Slots[slotIndex].Candidates))
	for _, candidate := range updated.Slots[slotIndex].Candidates {
		candidateIDs = append(candidateIDs, candidate.AssetID)
	}
	outcome := "blocked"
	if updated.Slots[slotIndex].Slot.Status == imageagent.SlotStatusAccepted {
		outcome = "accepted"
	}
	eventPayload, err := json.Marshal(struct {
		RunID        string                       `json:"run_id"`
		PlanRevision int64                        `json:"plan_revision"`
		SlotID       string                       `json:"slot_id"`
		Attempt      int                          `json:"attempt"`
		EffectPhase  imageagent.SlotEffectV3Phase `json:"effect_phase"`
	}{input.RunID, input.PlanRevision, input.Slot.ID, input.Attempt, effect.Phase})
	if err != nil {
		return err
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: commitID, ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "effect.recovery.reconciled", EventPayload: eventPayload,
		ExpectedRunVersion: current.Run.Version,
		RunMutation: &imageagent.RunMutation{
			Status: updated.Run.Status, CurrentNode: updated.Run.CurrentNode,
			ActivePlanRevision: updated.Run.ActivePlanRevision, Block: updated.Run.Block,
		},
		SlotMutation: &imageagent.SlotProjectionMutation{
			PlanRevision: input.PlanRevision,
			Result: imageagent.SlotResult{
				SlotID: input.Slot.ID, Attempt: input.Attempt, Status: updated.Slots[slotIndex].Slot.Status,
				CandidateAssetIDs: candidateIDs, ErrorCode: updated.Slots[slotIndex].ErrorCode,
			},
			Projection: updated.Slots[slotIndex],
			Attempt: imageagent.StepAttempt{
				TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID,
				PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
				Node:           updated.Run.CurrentNode,
				IdempotencyKey: fmt.Sprintf("%s:slot:%s:attempt:%d", commitID, input.Slot.ID, input.Attempt),
				Outcome:        outcome, ErrorCategory: updated.Slots[slotIndex].ErrorCode,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("commit image agent recovery parent projection: %w", err)
	}
	return nil
}

func effectRecoveryReconciliationCommitID(input EffectRecoveryWorkflowInput, phase imageagent.SlotEffectV3Phase) (string, error) {
	return updateFingerprint("effect_recovery_reconcile", struct {
		TenantID, OwnerUserID, RunID, SlotID string
		PlanRevision                         int64
		Attempt                              int
		Phase                                imageagent.SlotEffectV3Phase
	}{
		TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID,
		SlotID: input.Slot.ID, PlanRevision: input.PlanRevision, Attempt: input.Attempt, Phase: phase,
	})
}

func recoverableEffectIndex(effects []imageagent.RecoverableEffect, slotID string, attempt int) int {
	for index, effect := range effects {
		if effect.SlotID == strings.TrimSpace(slotID) && effect.Attempt == attempt {
			return index
		}
	}
	return -1
}

func recoveryParentBlock(effects []imageagent.RecoverableEffect) *imageagent.Block {
	if len(effects) == 0 {
		return nil
	}
	return &imageagent.Block{Code: effects[0].Code, Message: effects[0].Code, SlotID: effects[0].SlotID}
}

func recoveredSlotProjectionMatches(slot imageagent.SlotProjection, published imageagent.SlotEffectV3PublishedResult) bool {
	normalized, err := imageagent.NormalizeSlotEffectV3PublishedResult(published)
	if err != nil || slot.Slot.Status != imageagent.SlotStatusAccepted || slot.Attempt != normalized.Attempt || slot.ErrorCode != "" || len(slot.Candidates) != len(normalized.Candidates) {
		return false
	}
	for index, candidate := range normalized.Candidates {
		if slot.Candidates[index].AssetID != candidate.AssetID || slot.Candidates[index].SourceAssetID != candidate.SourceAssetID || slot.Candidates[index].DurableAsset != candidate.DurableAsset || (candidate.Width != 0 || candidate.Height != 0 || candidate.Operations != nil) && (slot.Candidates[index].Width != candidate.Width || slot.Candidates[index].Height != candidate.Height || !reflect.DeepEqual(slot.Candidates[index].Operations, candidate.Operations)) {
			return false
		}
	}
	return true
}

func recoveredBlockedProjectionMatches(projection imageagent.RunProjection, slotIndex, ownerIndex int, blockedCode string) bool {
	if projection.Slots[slotIndex].Slot.Status != imageagent.SlotStatusBlocked || projection.Slots[slotIndex].ErrorCode != blockedCode || projection.RecoverableEffects[ownerIndex].Code != blockedCode {
		return false
	}
	if projection.Run.Block == nil || strings.TrimSpace(projection.Run.Block.SlotID) != strings.TrimSpace(projection.Slots[slotIndex].Slot.ID) {
		return true
	}
	return projection.Run.Block.Code == blockedCode && projection.Run.Block.Message == blockedCode
}

func (a *Activities) persistMissingEffectRecoveryBlockedV3(ctx context.Context, input EffectRecoveryWorkflowInput, reservation imageagent.SlotEffectV3Reservation) (EffectRecoveryResult, error) {
	effect, _, err := a.slotEffectsV3.ReserveSlotProviderV3(ctx, reservation)
	if err != nil {
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	reservation.Policy = effect.Policy
	reservation.Quote = effect.Quote
	if effect.Phase == imageagent.SlotEffectV3RecoveryBlocked {
		return effectRecoveryBlockedResult(input), nil
	}
	return a.blockEffectRecoveryV3(ctx, input, reservation)
}

func (a *Activities) blockEffectRecoveryV3(ctx context.Context, input EffectRecoveryWorkflowInput, reservation imageagent.SlotEffectV3Reservation) (EffectRecoveryResult, error) {
	blocked, err := a.slotEffectsV3.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{
		Reservation: reservation,
		Phase:       imageagent.SlotEffectV3RecoveryBlocked,
		Code:        imageagent.SlotRecoveryBlockedCode,
	})
	if err != nil {
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	return EffectRecoveryResult{
		Outcome:     EffectRecoveryOutcomeRecoveryBlocked,
		Published:   imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
		EffectPhase: blocked.Phase,
		BlockedCode: blocked.BlockedCode,
	}, nil
}

func (a *Activities) publicationRecoveryError(message string, publication imageagent.PublicationClaim, cause error) error {
	delay := time.Until(publication.LeaseExpiresAt)
	if delay < 0 {
		delay = 0
	}
	delay += publicationLeaseRetrySafetyMargin
	maxDelay := a.publicationLeaseDuration + publicationLeaseRetrySafetyMargin
	if delay > maxDelay {
		delay = maxDelay
	}
	details := slotPublicationRecoveryDetails{
		RetryDelay: delay, LeaseExpiresAt: publication.LeaseExpiresAt,
		Owner: publication.Owner, Fence: publication.Fence,
	}
	return sdktemporal.NewApplicationErrorWithOptions(
		message,
		slotPublicationRecoveryErrorType,
		sdktemporal.ApplicationErrorOptions{
			NonRetryable: true, Cause: cause, Details: []interface{}{details}, NextRetryDelay: delay,
		},
	)
}

func effectRecoveryBlockedPhaseResult(outcome EffectRecoveryOutcome, phase imageagent.SlotEffectV3Phase, blockedCode string) EffectRecoveryResult {
	return EffectRecoveryResult{Outcome: outcome, EffectPhase: phase, BlockedCode: blockedCode}
}

func effectRecoveryResultFromError(err error) (EffectRecoveryResult, bool) {
	var applicationError *sdktemporal.ApplicationError
	if !errors.As(err, &applicationError) {
		return EffectRecoveryResult{}, false
	}
	switch applicationError.Type() {
	case slotProviderOutcomeUnknownCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode), true
	case slotStagingOutcomeUnknownCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode), true
	case slotPublicationOutcomeUnknownCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, imageagent.SlotEffectV3PublicationUnknown, slotPublicationOutcomeUnknownCode), true
	case imageagent.SlotReviewRequiredCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, imageagent.SlotEffectV3ReviewRequired, imageagent.SlotReviewRequiredCode), true
	case imageagent.SlotReviewTransportRequiredCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeReviewRequired, imageagent.SlotEffectV3ReviewTransportRequired, imageagent.SlotReviewTransportRequiredCode), true
	default:
		return EffectRecoveryResult{}, false
	}
}
