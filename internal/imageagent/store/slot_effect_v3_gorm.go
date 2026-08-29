package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
)

func (r *gormRepository) ReserveSlotProviderV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightBlock(transition); err != nil {
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
		decision, err := effectpolicy.Block(current, transition)
		if err != nil {
			return err
		}
		result = decision.Attempt
		if !decision.Changed {
			return nil
		}
		updates := map[string]any{"phase": string(decision.Attempt.Phase), "blocked_code": decision.Attempt.BlockedCode}
		if decision.Attempt.Phase == imageagent.SlotEffectV3RecoveryBlocked {
			updates["recovery_phase"] = string(decision.Attempt.RecoveryPhase)
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
		return nil
	})
	return result, err
}

func (r *gormRepository) RestoreRecoveryBlockedEffectV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
		decision, err := effectpolicy.RestoreRecoveryBlocked(current, reservation)
		if err != nil {
			return err
		}
		where := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Where("phase = ? AND recovery_phase = ?", string(imageagent.SlotEffectV3RecoveryBlocked), string(current.RecoveryPhase))
		updated := where.Updates(map[string]any{"phase": string(decision.Attempt.Phase), "blocked_code": decision.Attempt.BlockedCode, "recovery_phase": ""})
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
		decoded, decodeErr := decodeSlotEffectV3Record(row)
		if decodeErr != nil && !errors.Is(decodeErr, imageagent.ErrCorruptPersistedEffect) {
			return decodeErr
		}
		marker := row.CorruptionMarker
		if decodeErr != nil {
			marker = decoded.CorruptionMarker
		}
		decision, err := effectpolicy.FailClosedCorrupt(identity, marker, &decoded)
		if err != nil {
			return err
		}
		result = decision.Attempt
		if !decision.Changed {
			return nil
		}
		updated := tx.Model(&slotExternalEffectV3Record{}).Where(
			"tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND slot_id = ? AND attempt = ?",
			identity.TenantID, identity.OwnerUserID, identity.RunID, identity.PlanRevision, identity.SlotID, identity.Attempt,
		).Updates(map[string]any{
			"phase": string(decision.Attempt.Phase), "blocked_code": decision.Attempt.BlockedCode,
			"recovery_phase": "", "corruption_marker": decision.Attempt.CorruptionMarker,
		})
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
