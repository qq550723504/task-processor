package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
)

func (r *memoryRepository) ReserveSlotProviderV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemorySlotEffectScope(r, reservation.Identity); err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	runKey := scopeKey(reservation.Identity.RunScope)
	key := slotEffectKey(reservation.Identity)
	if existing, ok := r.slotEffectsV3[key]; ok {
		accounting, err := providerAccountingSnapshot(r.runs[runKey], r.reservedUsage[runKey])
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		decision, err := effectpolicy.ReserveProvider(&existing, reservation, accounting)
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		if err := r.persistMemoryProviderDecision(runKey, key, decision.AccountingDecision); err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		return cloneSlotEffectV3(decision.Attempt), decision.Acquired, nil
	}
	for _, existing := range r.slotEffectsV3 {
		if existing.Identity.RunScope == reservation.Identity.RunScope && existing.IdempotencyKey == reservation.IdempotencyKey {
			if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
				return imageagent.SlotEffectV3Attempt{}, false, err
			}
			return imageagent.SlotEffectV3Attempt{}, false, imageagent.ErrRevisionConflict
		}
	}
	accounting, err := providerAccountingSnapshot(r.runs[runKey], r.reservedUsage[runKey])
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	decision, err := effectpolicy.ReserveProvider(nil, reservation, accounting)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	if err := r.persistMemoryProviderDecision(runKey, key, decision.AccountingDecision); err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	return cloneSlotEffectV3(decision.Attempt), decision.Acquired, nil
}

func (r *memoryRepository) SettleSlotProviderV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemorySlotBudget(reservation, imageagent.SlotBudgetCommitted, receipt)
}

func (r *memoryRepository) RecordSlotProviderNotDispatchedV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := slotEffectKey(reservation.Identity)
	effect, ok := r.slotEffectsV3[key]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	runKey := scopeKey(reservation.Identity.RunScope)
	accounting, err := providerAccountingSnapshot(r.runs[runKey], r.reservedUsage[runKey])
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	decision, err := effectpolicy.RecordProviderNotDispatched(effect, reservation, accounting)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if err := r.persistMemoryProviderDecision(runKey, key, decision); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}

func (r *memoryRepository) ReleaseSlotProviderBudgetV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemorySlotBudget(reservation, imageagent.SlotBudgetReleased, imageagent.SlotUsageReceipt{})
}

func (r *memoryRepository) MarkSlotProviderBudgetUnknownV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemorySlotBudget(reservation, imageagent.SlotBudgetUnknown, imageagent.SlotUsageReceipt{})
}

func (r *memoryRepository) transitionMemorySlotBudget(reservation imageagent.SlotEffectV3Reservation, target imageagent.SlotBudgetStatus, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := slotEffectKey(reservation.Identity)
	effect, ok := r.slotEffectsV3[key]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	runKey := scopeKey(reservation.Identity.RunScope)
	accounting, err := providerAccountingSnapshot(r.runs[runKey], r.reservedUsage[runKey])
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var decision effectpolicy.AccountingDecision
	switch target {
	case imageagent.SlotBudgetCommitted:
		decision, err = effectpolicy.SettleProvider(effect, reservation, receipt, accounting, r.clock().UTC())
	case imageagent.SlotBudgetReleased:
		decision, err = effectpolicy.ReleaseProviderBudget(effect, reservation, accounting)
	case imageagent.SlotBudgetUnknown:
		decision, err = effectpolicy.MarkProviderBudgetUnknown(effect, reservation, accounting)
	default:
		err = imageagent.ErrValidation
	}
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if err := r.persistMemoryProviderDecision(runKey, key, decision); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}

func providerAccountingSnapshot(run imageagent.Run, reserved imageagent.UsageVector) (effectpolicy.AccountingSnapshot, error) {
	policy, err := run.Budget.Policy()
	if err != nil {
		return effectpolicy.AccountingSnapshot{}, imageagent.ErrRevisionConflict
	}
	committed, err := imageagent.UsageVectorFromBudgetUsage(run.Usage)
	if err != nil {
		return effectpolicy.AccountingSnapshot{}, err
	}
	return effectpolicy.AccountingSnapshot{
		Policy: policy, Committed: committed, Reserved: reserved, Elapsed: run.Usage.Elapsed, StartedAt: run.StartedAt,
	}, nil
}

func (r *memoryRepository) persistMemoryProviderDecision(runKey, effectKey string, decision effectpolicy.AccountingDecision) error {
	if decision.AccountingChanged {
		run := r.runs[runKey]
		usage, err := imageagent.BudgetUsageFromUsageVector(decision.Accounting.Committed, decision.Accounting.Elapsed)
		if err != nil {
			return err
		}
		run.Usage = usage
		r.runs[runKey] = run
		r.reservedUsage[runKey] = decision.Accounting.Reserved
	}
	if decision.Changed {
		r.slotEffectsV3[effectKey] = cloneSlotEffectV3(decision.Attempt)
	}
	return nil
}

func (r *memoryRepository) PrepareSlotStagingV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	normalizedManifest, _, err := effectpolicy.PreflightStagingManifest(manifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.PrepareStaging(existing, reservation, normalizedManifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(reservation.Identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}

func (r *memoryRepository) CommitSlotStagedV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation, fingerprint string) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.CommitStaged(existing, reservation, fingerprint)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(reservation.Identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}

func (r *memoryRepository) ClaimSlotPublicationV3(_ context.Context, request imageagent.PublicationClaimRequest) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, error) {
	request, err := effectpolicy.PreflightPublicationClaim(request)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(request.Reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.ClaimPublication(existing, request, r.clock().UTC())
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(decision.Attempt.Identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return cloneSlotEffectV3(decision.Attempt), decision.Claim, decision.Acquired, nil
}

func (r *memoryRepository) RenewSlotPublicationV3(_ context.Context, renewal imageagent.PublicationLeaseRenewal) (imageagent.PublicationClaim, error) {
	if err := effectpolicy.PreflightPublicationLeaseRenewal(renewal); err != nil {
		return imageagent.PublicationClaim{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(renewal.Identity)]
	if !ok {
		return imageagent.PublicationClaim{}, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.RenewPublication(existing, renewal, r.clock().UTC())
	if err != nil {
		return imageagent.PublicationClaim{}, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(decision.Attempt.Identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return decision.Claim, nil
}

func (r *memoryRepository) CompleteSlotPublicationV3(_ context.Context, completion imageagent.PublicationCompletion) (imageagent.SlotEffectV3Attempt, error) {
	completion, err := effectpolicy.PreflightPublicationCompletion(completion)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(completion.Reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.CompletePublication(existing, completion)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(decision.Attempt.Identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}

func (r *memoryRepository) BlockSlotEffectV3(_ context.Context, transition imageagent.SlotEffectV3BlockTransition) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateBlockTransitionV3(transition); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(transition.Reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if !sameSlotEffectV3Reservation(existing, transition.Reservation) || existing.Phase == imageagent.SlotEffectV3PublicationComplete {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase == transition.Phase && existing.BlockedCode == transition.Code {
		if transition.Phase != imageagent.SlotEffectV3PublicationUnknown || (existing.Publication.Owner == transition.Owner && existing.Publication.Fence == transition.Fence) {
			return cloneSlotEffectV3(existing), nil
		}
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase != imageagent.SlotEffectV3RecoveryBlocked && isBlockedV3Phase(existing.Phase) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if !canBlockV3(existing.Phase, transition.Phase) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase == imageagent.SlotEffectV3PublicationUnknown && (existing.Publication.Owner != transition.Owner || existing.Publication.Fence != transition.Fence) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase == imageagent.SlotEffectV3RecoveryBlocked {
		existing.RecoveryPhase = existing.Phase
	}
	existing.Phase = transition.Phase
	existing.BlockedCode = transition.Code
	r.slotEffectsV3[slotEffectKey(existing.Identity)] = cloneSlotEffectV3(existing)
	return cloneSlotEffectV3(existing), nil
}

func (r *memoryRepository) RestoreRecoveryBlockedEffectV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := slotEffectKey(reservation.Identity)
	effect, ok := r.slotEffectsV3[key]
	if !ok || !sameSlotEffectV3Reservation(effect, reservation) || effect.Phase != imageagent.SlotEffectV3RecoveryBlocked || !isRedrivableRecoveryPhase(effect.RecoveryPhase) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	effect.Phase = effect.RecoveryPhase
	effect.RecoveryPhase = ""
	effect.BlockedCode = ""
	r.slotEffectsV3[key] = cloneSlotEffectV3(effect)
	return cloneSlotEffectV3(effect), nil
}

func (r *memoryRepository) GetSlotExternalEffectV3(_ context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectIdentity(identity); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(identity.RunScope)]; !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	effect, ok := r.slotEffectsV3[slotEffectKey(identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(effect); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	return cloneSlotEffectV3(effect), nil
}

func (r *memoryRepository) BlockCorruptSlotEffectV3(_ context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectIdentity(identity); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(identity.RunScope)]; !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	effect, ok := r.slotEffectsV3[slotEffectKey(identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	if effect.Phase != imageagent.SlotEffectV3RecoveryBlocked || effect.BlockedCode != imageagent.SlotRecoveryBlockedCode || strings.TrimSpace(effect.CorruptionMarker) == "" {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrCorruptPersistedEffect
	}
	return cloneSlotEffectV3(effect), nil
}

func (r *gormRepository) ReserveSlotProviderV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	var result imageagent.SlotEffectV3Attempt
	claimed := false
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		lockedRun, err := r.findRunForUpdate(ctx, tx, reservation.Identity.RunScope)
		if err != nil {
			return err
		}
		accounting, err := providerAccountingSnapshotFromRecord(lockedRun)
		if err != nil {
			return err
		}
		if existingRow, findErr := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity); findErr == nil {
			existing, decodeErr := decodeSlotEffectV3Record(existingRow)
			if decodeErr != nil {
				return decodeErr
			}
			decision, policyErr := effectpolicy.ReserveProvider(&existing, reservation, accounting)
			if policyErr != nil {
				return policyErr
			}
			if persistErr := persistGormProviderReservation(ctx, tx, reservation.Identity.RunScope, existing, decision); persistErr != nil {
				return persistErr
			}
			result = decision.Attempt
			claimed = decision.Acquired
			return nil
		} else if !errors.Is(findErr, imageagent.ErrRunNotFound) {
			return findErr
		}
		if collision, collisionErr := findSlotEffectV3ByIdempotencyForUpdate(ctx, tx, reservation); collisionErr == nil {
			if collision.Identity != reservation.Identity {
				return imageagent.ErrRevisionConflict
			}
		} else if !errors.Is(collisionErr, imageagent.ErrRunNotFound) {
			return collisionErr
		}
		decision, err := effectpolicy.ReserveProvider(nil, reservation, accounting)
		if err != nil {
			return err
		}
		now, err := databaseNow(ctx, tx)
		if err != nil {
			return err
		}
		row := slotEffectV3RecordFromProviderDecision(decision, now)
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			if persistErr := persistGormProviderAccounting(tx, reservation.Identity.RunScope, decision.AccountingDecision); persistErr != nil {
				return persistErr
			}
			claimed = decision.Acquired
			result = decision.Attempt
			return nil
		}
		existing, err := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			if errors.Is(err, imageagent.ErrRunNotFound) {
				collision, collisionErr := findSlotEffectV3ByIdempotencyForUpdate(ctx, tx, reservation)
				if collisionErr != nil {
					return collisionErr
				}
				if collision.Identity != reservation.Identity {
					return imageagent.ErrRevisionConflict
				}
			}
			return err
		}
		result, err = decodeSlotEffectV3Record(existing)
		if err != nil {
			return err
		}
		collisionDecision, policyErr := effectpolicy.ReserveProvider(&result, reservation, accounting)
		if policyErr != nil {
			return policyErr
		}
		if persistErr := persistGormProviderReservation(ctx, tx, reservation.Identity.RunScope, result, collisionDecision); persistErr != nil {
			return persistErr
		}
		result = collisionDecision.Attempt
		claimed = collisionDecision.Acquired
		return nil
	})
	return result, claimed, err
}

func (r *gormRepository) SettleSlotProviderV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormSlotBudget(ctx, reservation, imageagent.SlotBudgetCommitted, receipt)
}

func (r *gormRepository) RecordSlotProviderNotDispatchedV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		runRow, err := r.findRunForUpdate(ctx, tx, reservation.Identity.RunScope)
		if err != nil {
			return err
		}
		effectRow, err := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		effect, err := decodeSlotEffectV3Record(effectRow)
		if err != nil {
			return err
		}
		accounting, err := providerAccountingSnapshotFromRecord(runRow)
		if err != nil {
			return err
		}
		decision, err := effectpolicy.RecordProviderNotDispatched(effect, reservation, accounting)
		if err != nil {
			return err
		}
		if err := persistGormProviderAccounting(tx, reservation.Identity.RunScope, decision); err != nil {
			return err
		}
		if decision.Changed {
			updates := map[string]any{"phase": string(decision.Attempt.Phase), "budget_status": string(decision.Attempt.BudgetStatus)}
			if effect.BudgetStatus == imageagent.SlotBudgetReserved && decision.Attempt.BudgetStatus == imageagent.SlotBudgetReleased {
				now, nowErr := databaseNow(ctx, tx)
				if nowErr != nil {
					return nowErr
				}
				updates["budget_released_at"] = now
			}
			if updateErr := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Updates(updates).Error; updateErr != nil {
				return updateErr
			}
		}
		result = decision.Attempt
		return nil
	})
	return result, err
}

func (r *gormRepository) ReleaseSlotProviderBudgetV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormSlotBudget(ctx, reservation, imageagent.SlotBudgetReleased, imageagent.SlotUsageReceipt{})
}

func (r *gormRepository) MarkSlotProviderBudgetUnknownV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormSlotBudget(ctx, reservation, imageagent.SlotBudgetUnknown, imageagent.SlotUsageReceipt{})
}

func (r *gormRepository) transitionGormSlotBudget(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, target imageagent.SlotBudgetStatus, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		runRow, err := r.findRunForUpdate(ctx, tx, reservation.Identity.RunScope)
		if err != nil {
			return err
		}
		effectRow, err := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		effect, err := decodeSlotEffectV3Record(effectRow)
		if err != nil {
			return err
		}
		accounting, err := providerAccountingSnapshotFromRecord(runRow)
		if err != nil {
			return err
		}
		now, err := databaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var decision effectpolicy.AccountingDecision
		switch target {
		case imageagent.SlotBudgetCommitted:
			decision, err = effectpolicy.SettleProvider(effect, reservation, receipt, accounting, now)
		case imageagent.SlotBudgetReleased:
			decision, err = effectpolicy.ReleaseProviderBudget(effect, reservation, accounting)
		case imageagent.SlotBudgetUnknown:
			decision, err = effectpolicy.MarkProviderBudgetUnknown(effect, reservation, accounting)
		default:
			err = imageagent.ErrValidation
		}
		if err != nil {
			return err
		}
		if err := persistGormProviderAccounting(tx, reservation.Identity.RunScope, decision); err != nil {
			return err
		}
		if decision.Changed {
			updates := map[string]any{"budget_status": string(decision.Attempt.BudgetStatus)}
			switch target {
			case imageagent.SlotBudgetCommitted:
				receiptJSON, marshalErr := json.Marshal(decision.Attempt.Receipt)
				if marshalErr != nil {
					return marshalErr
				}
				updates["usage_receipt_json"] = receiptJSON
				updates["budget_settled_at"] = now
			case imageagent.SlotBudgetReleased:
				updates["budget_released_at"] = now
			case imageagent.SlotBudgetUnknown:
				updates["budget_unknown_at"] = now
			}
			if updateErr := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Updates(updates).Error; updateErr != nil {
				return updateErr
			}
		}
		result = decision.Attempt
		return nil
	})
	return result, err
}

func providerAccountingSnapshotFromRecord(row runRecord) (effectpolicy.AccountingSnapshot, error) {
	run, err := recordToRun(row)
	if err != nil {
		return effectpolicy.AccountingSnapshot{}, err
	}
	reserved, err := decodeReservedUsage(row.ReservedUsageJSON)
	if err != nil {
		return effectpolicy.AccountingSnapshot{}, err
	}
	return providerAccountingSnapshot(run, reserved)
}

func persistGormProviderReservation(ctx context.Context, tx *gorm.DB, scope imageagent.RunScope, current imageagent.SlotEffectV3Attempt, decision effectpolicy.ProviderReservationDecision) error {
	if err := persistGormProviderAccounting(tx, scope, decision.AccountingDecision); err != nil {
		return err
	}
	if !decision.Changed {
		return nil
	}
	updates := map[string]any{"phase": string(decision.Attempt.Phase), "budget_status": string(decision.Attempt.BudgetStatus)}
	if current.Phase == imageagent.SlotEffectV3ProviderNotDispatched && decision.Attempt.Phase == imageagent.SlotEffectV3ProviderClaimed {
		now, err := databaseNow(ctx, tx)
		if err != nil {
			return err
		}
		updates["provider_claimed_at"] = now
	}
	if current.BudgetStatus == imageagent.SlotBudgetReleased && decision.Attempt.BudgetStatus == imageagent.SlotBudgetReserved {
		updates["budget_released_at"] = nil
	}
	return slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), decision.Attempt.Identity).Updates(updates).Error
}

func persistGormProviderAccounting(tx *gorm.DB, scope imageagent.RunScope, decision effectpolicy.AccountingDecision) error {
	if !decision.AccountingChanged {
		return nil
	}
	reservedJSON, err := json.Marshal(decision.Accounting.Reserved)
	if err != nil {
		return err
	}
	usage, err := imageagent.BudgetUsageFromUsageVector(decision.Accounting.Committed, decision.Accounting.Elapsed)
	if err != nil {
		return err
	}
	usageJSON, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	return runScopeWhere(tx.Model(&runRecord{}), scope).Updates(map[string]any{"reserved_usage_json": reservedJSON, "usage_json": usageJSON}).Error
}

func (r *gormRepository) PrepareSlotStagingV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	normalizedManifest, _, err := effectpolicy.PreflightStagingManifest(manifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err = withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		decision, err := effectpolicy.PrepareStaging(current, reservation, normalizedManifest)
		if err != nil {
			return err
		}
		if !decision.Changed {
			result = decision.Attempt
			return nil
		}
		encoded, err := json.Marshal(decision.Attempt.StagingManifest)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Where("phase = ?", string(imageagent.SlotEffectV3ProviderClaimed)).Updates(map[string]any{"phase": string(decision.Attempt.Phase), "staging_manifest_json": encoded, "staging_manifest_fingerprint": decision.Attempt.StagingManifestFingerprint, "staging_prepared_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		result = decision.Attempt
		return nil
	})
	return result, err
}

func (r *gormRepository) CommitSlotStagedV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, fingerprint string) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		decision, err := effectpolicy.CommitStaged(current, reservation, fingerprint)
		if err != nil {
			return err
		}
		if !decision.Changed {
			result = decision.Attempt
			return nil
		}
		now := time.Now().UTC()
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Where("phase = ?", string(imageagent.SlotEffectV3StagingPrepared)).Updates(map[string]any{"phase": string(decision.Attempt.Phase), "staged_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		result = decision.Attempt
		return nil
	})
	return result, err
}

func (r *gormRepository) ClaimSlotPublicationV3(ctx context.Context, request imageagent.PublicationClaimRequest) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, error) {
	request, err := effectpolicy.PreflightPublicationClaim(request)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	var result imageagent.SlotEffectV3Attempt
	var claim imageagent.PublicationClaim
	claimed := false
	err = withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, request.Reservation.Identity)
		if err != nil {
			return err
		}
		now, err := databaseNow(ctx, tx)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		decision, err := effectpolicy.ClaimPublication(current, request, now)
		if err != nil {
			return err
		}
		result, claim, claimed = decision.Attempt, decision.Claim, decision.Acquired
		if !decision.Changed {
			return nil
		}
		finalJSON, err := json.Marshal(decision.Attempt.FinalManifest)
		if err != nil {
			return err
		}
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), request.Reservation.Identity).Updates(map[string]any{"phase": string(decision.Attempt.Phase), "publication_owner": decision.Attempt.Publication.Owner, "publication_lease_expires_at": decision.Attempt.Publication.LeaseExpiresAt, "publication_fence": decision.Attempt.Publication.Fence, "publication_fingerprint": decision.Attempt.PublicationFingerprint, "final_manifest_json": finalJSON})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		return nil
	})
	return result, claim, claimed, err
}

func (r *gormRepository) RenewSlotPublicationV3(ctx context.Context, renewal imageagent.PublicationLeaseRenewal) (imageagent.PublicationClaim, error) {
	if err := effectpolicy.PreflightPublicationLeaseRenewal(renewal); err != nil {
		return imageagent.PublicationClaim{}, err
	}
	var claim imageagent.PublicationClaim
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, renewal.Identity)
		if err != nil {
			return err
		}
		now, err := databaseNow(ctx, tx)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		decision, err := effectpolicy.RenewPublication(current, renewal, now)
		if err != nil {
			return err
		}
		claim = decision.Claim
		if !decision.Changed {
			return nil
		}
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), renewal.Identity).Where("phase = ? AND publication_owner = ? AND publication_fence = ?", string(imageagent.SlotEffectV3PublicationClaimed), renewal.Owner, renewal.Fence).Updates(map[string]any{"publication_lease_expires_at": claim.LeaseExpiresAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		return nil
	})
	return claim, err
}

func (r *gormRepository) CompleteSlotPublicationV3(ctx context.Context, completion imageagent.PublicationCompletion) (imageagent.SlotEffectV3Attempt, error) {
	completion, err := effectpolicy.PreflightPublicationCompletion(completion)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	encoded, err := json.Marshal(completion.Published)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err = withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, completion.Reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		decision, err := effectpolicy.CompletePublication(current, completion)
		if err != nil {
			return err
		}
		result = decision.Attempt
		if !decision.Changed {
			return nil
		}
		now := time.Now().UTC()
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), completion.Reservation.Identity).Where("phase = ? AND publication_owner = ? AND publication_fence = ?", string(imageagent.SlotEffectV3PublicationClaimed), completion.Owner, completion.Fence).Updates(map[string]any{"phase": string(imageagent.SlotEffectV3PublicationComplete), "result_fingerprint": completion.ResultFingerprint, "published_json": encoded, "published_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		return nil
	})
	return result, err
}

func (r *gormRepository) BlockSlotEffectV3(ctx context.Context, transition imageagent.SlotEffectV3BlockTransition) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateBlockTransitionV3(transition); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, transition.Reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		if !sameSlotEffectV3Reservation(current, transition.Reservation) || current.Phase == imageagent.SlotEffectV3PublicationComplete {
			return imageagent.ErrRevisionConflict
		}
		if current.Phase == transition.Phase && current.BlockedCode == transition.Code {
			if transition.Phase != imageagent.SlotEffectV3PublicationUnknown || (current.Publication.Owner == transition.Owner && current.Publication.Fence == transition.Fence) {
				result = current
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
		if transition.Phase != imageagent.SlotEffectV3RecoveryBlocked && isBlockedV3Phase(current.Phase) {
			return imageagent.ErrRevisionConflict
		}
		if !canBlockV3(current.Phase, transition.Phase) {
			return imageagent.ErrRevisionConflict
		}
		if transition.Phase == imageagent.SlotEffectV3PublicationUnknown && (current.Publication.Owner != transition.Owner || current.Publication.Fence != transition.Fence) {
			return imageagent.ErrRevisionConflict
		}
		updates := map[string]any{"phase": string(transition.Phase), "blocked_code": transition.Code}
		if transition.Phase == imageagent.SlotEffectV3RecoveryBlocked {
			updates["recovery_phase"] = string(current.Phase)
			current.RecoveryPhase = current.Phase
		}
		where := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), transition.Reservation.Identity)
		if transition.Phase == imageagent.SlotEffectV3PublicationUnknown {
			where = where.Where("publication_owner = ? AND publication_fence = ?", transition.Owner, transition.Fence)
		}
		updated := where.Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Phase = transition.Phase
		current.BlockedCode = transition.Code
		result = current
		return nil
	})
	return result, err
}

func (r *gormRepository) RestoreRecoveryBlockedEffectV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		if !sameSlotEffectV3Reservation(current, reservation) || current.Phase != imageagent.SlotEffectV3RecoveryBlocked || !isRedrivableRecoveryPhase(current.RecoveryPhase) {
			return imageagent.ErrRevisionConflict
		}
		where := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Where("phase = ? AND recovery_phase = ?", string(imageagent.SlotEffectV3RecoveryBlocked), string(current.RecoveryPhase))
		updated := where.Updates(map[string]any{"phase": string(current.RecoveryPhase), "blocked_code": "", "recovery_phase": ""})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Phase = current.RecoveryPhase
		current.RecoveryPhase = ""
		current.BlockedCode = ""
		result = current
		return nil
	})
	return result, err
}

func (r *gormRepository) GetSlotExternalEffectV3(ctx context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectIdentity(identity); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if _, err := r.findRun(ctx, r.db, identity.RunScope); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	row, err := findSlotEffectV3(ctx, r.db, identity)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	return decodeSlotEffectV3Record(row)
}

func (r *gormRepository) BlockCorruptSlotEffectV3(ctx context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error) {
	if err := validateSlotEffectIdentity(identity); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	var result imageagent.SlotEffectV3Attempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		if _, err := r.findRunForUpdate(ctx, tx, identity.RunScope); err != nil {
			return err
		}
		row, err := findSlotEffectV3ForUpdate(ctx, tx, identity)
		if err != nil {
			return err
		}
		if row.Phase == string(imageagent.SlotEffectV3RecoveryBlocked) && strings.TrimSpace(row.CorruptionMarker) != "" {
			decoded, decodeErr := decodeSlotEffectV3Record(row)
			if decodeErr != nil {
				return decodeErr
			}
			result = decoded
			return nil
		}
		decoded, decodeErr := decodeSlotEffectV3Record(row)
		if decodeErr == nil {
			return imageagent.ErrCorruptPersistedEffect
		}
		if !errors.Is(decodeErr, imageagent.ErrCorruptPersistedEffect) {
			return decodeErr
		}
		marker := decoded.CorruptionMarker
		if strings.TrimSpace(marker) == "" {
			return imageagent.ErrCorruptPersistedEffect
		}
		updated := tx.Model(&slotExternalEffectV3Record{}).Where(
			"tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND slot_id = ? AND attempt = ?",
			identity.TenantID, identity.OwnerUserID, identity.RunID, identity.PlanRevision, identity.SlotID, identity.Attempt,
		).Updates(map[string]any{
			"phase": string(imageagent.SlotEffectV3RecoveryBlocked), "blocked_code": imageagent.SlotRecoveryBlockedCode, "corruption_marker": marker,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		row.Phase = string(imageagent.SlotEffectV3RecoveryBlocked)
		row.BlockedCode = imageagent.SlotRecoveryBlockedCode
		row.CorruptionMarker = marker
		result, err = decodeSlotEffectV3Record(row)
		return err
	})
	return result, err
}

func validateSlotEffectV3Reservation(reservation imageagent.SlotEffectV3Reservation) error {
	if err := validateSlotEffectIdentity(reservation.Identity); err != nil {
		return err
	}
	if strings.TrimSpace(reservation.IdempotencyKey) == "" || strings.TrimSpace(reservation.InputFingerprint) == "" {
		return imageagent.ErrValidation
	}
	if reservation.Quote.Fingerprint != "" {
		if err := imageagent.ValidateSlotUsageQuote(reservation.Quote); err != nil {
			return err
		}
		if err := reservation.Policy.Allows(imageagent.UsageVector{}, imageagent.UsageVector{}, imageagent.UsageVector{}); err != nil {
			return err
		}
	}
	return nil
}

func validateBlockTransitionV3(transition imageagent.SlotEffectV3BlockTransition) error {
	if err := validateSlotEffectV3Reservation(transition.Reservation); err != nil {
		return err
	}
	if !isBlockedV3Phase(transition.Phase) || strings.TrimSpace(transition.Code) == "" {
		return imageagent.ErrValidation
	}
	if _, err := imageagent.SlotEffectV3BlockedPolicyFor(transition.Phase, transition.Code); err != nil {
		return err
	}
	if transition.Phase == imageagent.SlotEffectV3PublicationUnknown && (strings.TrimSpace(transition.Owner) == "" || transition.Fence <= 0) {
		return imageagent.ErrValidation
	}
	return nil
}

func isBlockedV3Phase(phase imageagent.SlotEffectV3Phase) bool {
	return phase == imageagent.SlotEffectV3ProviderUnknown || phase == imageagent.SlotEffectV3StagingUnknown || phase == imageagent.SlotEffectV3PublicationUnknown || phase == imageagent.SlotEffectV3RecoveryBlocked
}

func isRedrivableRecoveryPhase(phase imageagent.SlotEffectV3Phase) bool {
	// ProviderClaimed and ProviderNotDispatched can only be resumed by a new
	// provider dispatch, which an explicit recovery redrive must never perform.
	return phase == imageagent.SlotEffectV3StagingPrepared ||
		phase == imageagent.SlotEffectV3ArtifactStaged ||
		phase == imageagent.SlotEffectV3PublicationClaimed ||
		phase == imageagent.SlotEffectV3ProviderUnknown ||
		phase == imageagent.SlotEffectV3StagingUnknown ||
		phase == imageagent.SlotEffectV3PublicationUnknown
}

func canBlockV3(current, blocked imageagent.SlotEffectV3Phase) bool {
	switch blocked {
	case imageagent.SlotEffectV3ProviderUnknown:
		return current == imageagent.SlotEffectV3ProviderClaimed
	case imageagent.SlotEffectV3StagingUnknown:
		return current == imageagent.SlotEffectV3StagingPrepared
	case imageagent.SlotEffectV3PublicationUnknown:
		return current == imageagent.SlotEffectV3PublicationClaimed
	case imageagent.SlotEffectV3RecoveryBlocked:
		return current == imageagent.SlotEffectV3ProviderClaimed ||
			current == imageagent.SlotEffectV3ProviderNotDispatched ||
			current == imageagent.SlotEffectV3StagingPrepared ||
			current == imageagent.SlotEffectV3ArtifactStaged ||
			current == imageagent.SlotEffectV3PublicationClaimed ||
			current == imageagent.SlotEffectV3ProviderUnknown ||
			current == imageagent.SlotEffectV3StagingUnknown ||
			current == imageagent.SlotEffectV3PublicationUnknown
	default:
		return false
	}
}

func sameSlotEffectV3Reservation(effect imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) bool {
	return effect.Identity == reservation.Identity && effect.IdempotencyKey == reservation.IdempotencyKey && effect.InputFingerprint == reservation.InputFingerprint && effect.Quote.Fingerprint == reservation.Quote.Fingerprint
}

func findSlotEffectV3ForUpdate(ctx context.Context, db *gorm.DB, identity imageagent.SlotExternalEffectIdentity) (slotExternalEffectV3Record, error) {
	return findSlotEffectV3(ctx, db.Clauses(clause.Locking{Strength: "UPDATE"}), identity)
}

func findSlotEffectV3ByIdempotencyForUpdate(ctx context.Context, db *gorm.DB, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	var row slotExternalEffectV3Record
	identity := reservation.Identity
	err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND idempotency_key = ?", identity.TenantID, identity.OwnerUserID, identity.RunID, reservation.IdempotencyKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, fmt.Errorf("get image agent v3 slot external effect by idempotency key: %w", err)
	}
	return decodeSlotEffectV3Record(row)
}

func findSlotEffectV3(ctx context.Context, db *gorm.DB, identity imageagent.SlotExternalEffectIdentity) (slotExternalEffectV3Record, error) {
	var row slotExternalEffectV3Record
	err := slotEffectV3IdentityWhere(db.WithContext(ctx), identity).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, imageagent.ErrRunNotFound
	}
	if err != nil {
		return row, fmt.Errorf("get image agent v3 slot external effect: %w", err)
	}
	return row, nil
}

func slotEffectV3IdentityWhere(db *gorm.DB, identity imageagent.SlotExternalEffectIdentity) *gorm.DB {
	return db.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND slot_id = ? AND attempt = ?", identity.TenantID, identity.OwnerUserID, identity.RunID, identity.PlanRevision, identity.SlotID, identity.Attempt)
}

func runScopeWhere(db *gorm.DB, scope imageagent.RunScope) *gorm.DB {
	return db.Where("tenant_id = ? AND owner_user_id = ? AND id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID)
}

func slotEffectV3RecordFromProviderDecision(decision effectpolicy.ProviderReservationDecision, claimedAt time.Time) slotExternalEffectV3Record {
	attempt := decision.Attempt
	identity := attempt.Identity
	policyJSON, _ := json.Marshal(attempt.Policy)
	quoteJSON, _ := json.Marshal(attempt.Quote)
	return slotExternalEffectV3Record{TenantID: identity.TenantID, OwnerUserID: identity.OwnerUserID, RunID: identity.RunID, PlanRevision: identity.PlanRevision, SlotID: identity.SlotID, Attempt: identity.Attempt, IdempotencyKey: attempt.IdempotencyKey, InputFingerprint: attempt.InputFingerprint, Phase: string(attempt.Phase), BudgetStatus: string(attempt.BudgetStatus), BudgetPolicyJSON: policyJSON, UsageQuoteJSON: quoteJSON, UsageQuoteFingerprint: attempt.Quote.Fingerprint, PricingVersion: attempt.Quote.PricingVersion, ProviderClaimedAt: claimedAt}
}

func decodeSlotEffectV3Record(row slotExternalEffectV3Record) (imageagent.SlotEffectV3Attempt, error) {
	result := slotEffectV3FromRecord(row)
	result.CorruptionMarker = row.CorruptionMarker
	if row.Phase == string(imageagent.SlotEffectV3RecoveryBlocked) && strings.TrimSpace(row.CorruptionMarker) != "" {
		if row.BlockedCode != imageagent.SlotRecoveryBlockedCode {
			return result, fmt.Errorf("%w: corrupt effect has invalid recovery block code", imageagent.ErrInvalidPersistedPolicy)
		}
		return result, nil
	}
	if len(row.StagingManifestJSON) > 0 {
		if err := json.Unmarshal(row.StagingManifestJSON, &result.StagingManifest); err != nil {
			return result, fmt.Errorf("decode v3 staging manifest: %w", err)
		}
	}
	if len(row.FinalManifestJSON) > 0 {
		if err := json.Unmarshal(row.FinalManifestJSON, &result.FinalManifest); err != nil {
			return result, fmt.Errorf("decode v3 final manifest: %w", err)
		}
	}
	if len(row.PublishedJSON) > 0 {
		if err := json.Unmarshal(row.PublishedJSON, &result.Published); err != nil {
			return result, fmt.Errorf("decode v3 published result: %w", err)
		}
	}
	if len(row.BudgetPolicyJSON) > 0 {
		if err := json.Unmarshal(row.BudgetPolicyJSON, &result.Policy); err != nil {
			result.CorruptionMarker = persistedEffectCorruptionMarker("budget_policy_json", row.BudgetPolicyJSON)
			return result, fmt.Errorf("%w: decode v3 budget policy: %w", imageagent.ErrCorruptPersistedEffect, err)
		}
	}
	if len(row.UsageQuoteJSON) > 0 {
		if err := json.Unmarshal(row.UsageQuoteJSON, &result.Quote); err != nil {
			result.CorruptionMarker = persistedEffectCorruptionMarker("usage_quote_json", row.UsageQuoteJSON)
			return result, fmt.Errorf("%w: decode v3 usage quote: %w", imageagent.ErrCorruptPersistedEffect, err)
		}
	}
	if len(row.UsageReceiptJSON) > 0 {
		if err := json.Unmarshal(row.UsageReceiptJSON, &result.Receipt); err != nil {
			return result, fmt.Errorf("decode v3 usage receipt: %w", err)
		}
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(result); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	return result, nil
}

func persistedEffectCorruptionMarker(field string, payload []byte) string {
	digest := sha256.Sum256(payload)
	return field + ":sha256:" + hex.EncodeToString(digest[:])
}

func slotEffectV3FromRecord(row slotExternalEffectV3Record) imageagent.SlotEffectV3Attempt {
	claim := imageagent.PublicationClaim{Owner: row.PublicationOwner, Fence: row.PublicationFence}
	if row.PublicationLeaseExpiresAt != nil {
		claim.LeaseExpiresAt = row.PublicationLeaseExpiresAt.UTC()
	}
	return imageagent.SlotEffectV3Attempt{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: row.TenantID, OwnerUserID: row.OwnerUserID, RunID: row.RunID}, PlanRevision: row.PlanRevision, SlotID: row.SlotID, Attempt: row.Attempt}, IdempotencyKey: row.IdempotencyKey, InputFingerprint: row.InputFingerprint, Phase: imageagent.SlotEffectV3Phase(row.Phase), StagingManifestFingerprint: row.StagingManifestFingerprint, Publication: claim, PublicationFingerprint: row.PublicationFingerprint, ResultFingerprint: row.ResultFingerprint, BlockedCode: row.BlockedCode, RecoveryPhase: imageagent.SlotEffectV3Phase(row.RecoveryPhase), CorruptionMarker: row.CorruptionMarker, BudgetStatus: imageagent.SlotBudgetStatus(row.BudgetStatus)}
}

func decodeReservedUsage(encoded []byte) (imageagent.UsageVector, error) {
	if len(encoded) == 0 || string(encoded) == "{}" || string(encoded) == "null" {
		return imageagent.UsageVector{}, nil
	}
	var usage imageagent.UsageVector
	if err := json.Unmarshal(encoded, &usage); err != nil {
		return imageagent.UsageVector{}, fmt.Errorf("decode reserved image agent usage: %w", err)
	}
	if _, err := imageagent.CheckedAddUsage(usage, imageagent.UsageVector{}); err != nil {
		return imageagent.UsageVector{}, err
	}
	return usage, nil
}

func databaseNow(ctx context.Context, tx *gorm.DB) (time.Time, error) {
	var value string
	query := "SELECT CURRENT_TIMESTAMP"
	if tx.Dialector.Name() == "postgres" {
		query = "SELECT clock_timestamp()"
	}
	if err := tx.WithContext(ctx).Raw(query).Scan(&value).Error; err != nil {
		return time.Time{}, fmt.Errorf("read database current time: %w", err)
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05"} {
		if now, err := time.Parse(layout, value); err == nil {
			return now.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse database current time %q", value)
}

func cloneSlotEffectV3(effect imageagent.SlotEffectV3Attempt) imageagent.SlotEffectV3Attempt {
	effect.StagingManifest = cloneStagingManifest(effect.StagingManifest)
	effect.FinalManifest = cloneFinalManifest(effect.FinalManifest)
	effect.Published = cloneSlotEffectV3PublishedResult(effect.Published)
	effect.Quote = cloneSlotUsageQuote(effect.Quote)
	effect.Receipt = cloneSlotUsageReceipt(effect.Receipt)
	return effect
}

func cloneSlotUsageQuote(quote imageagent.SlotUsageQuote) imageagent.SlotUsageQuote {
	quote.Operations = append([]imageagent.SlotUsageOperation(nil), quote.Operations...)
	return quote
}

func cloneSlotUsageReceipt(receipt imageagent.SlotUsageReceipt) imageagent.SlotUsageReceipt {
	receipt.ProviderRequestIDs = append([]string(nil), receipt.ProviderRequestIDs...)
	return receipt
}

func cloneSlotEffectV3PublishedResult(result imageagent.SlotEffectV3PublishedResult) imageagent.SlotEffectV3PublishedResult {
	result.Candidates = append([]imageagent.SlotEffectV3AssetCandidate(nil), result.Candidates...)
	return result
}

func cloneStagingManifest(manifest imageagent.StagingManifest) imageagent.StagingManifest {
	manifest.Assets = cloneStagedAssetRefs(manifest.Assets)
	if manifest.ProviderMetadata != nil {
		manifest.ProviderMetadata = cloneMetadata(manifest.ProviderMetadata)
	}
	return manifest
}

func cloneFinalManifest(manifest imageagent.FinalManifest) imageagent.FinalManifest {
	manifest.Assets = clonePublishedAssetRefs(manifest.Assets)
	return manifest
}

func clonePublishedAssetRefs(assets []imageagent.PublishedAssetRef) []imageagent.PublishedAssetRef {
	cloned := make([]imageagent.PublishedAssetRef, len(assets))
	for index, asset := range assets {
		if asset.Operations != nil {
			asset.Operations = append([]string{}, asset.Operations...)
		}
		cloned[index] = asset
	}
	return cloned
}

func cloneStagedAssetRefs(assets []imageagent.StagedAssetRef) []imageagent.StagedAssetRef {
	cloned := make([]imageagent.StagedAssetRef, len(assets))
	for index, asset := range assets {
		if asset.Operations != nil {
			asset.Operations = append([]string{}, asset.Operations...)
		}
		cloned[index] = asset
	}
	return cloned
}

var _ imageagent.SlotExternalEffectV3Repository = (*memoryRepository)(nil)
var _ imageagent.SlotExternalEffectV3Repository = (*gormRepository)(nil)
