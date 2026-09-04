package store

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
)

func (r *gormRepository) ReserveSlotReviewV3(ctx context.Context, reservation imageagent.SlotReviewUsageReservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	var result imageagent.SlotEffectV3Attempt
	acquired := false
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
		decision, err := effectpolicy.ReserveReview(&effect, reservation, accounting)
		if err != nil {
			return err
		}
		if err := persistGormProviderAccounting(tx, reservation.Identity.RunScope, decision.AccountingDecision); err != nil {
			return err
		}
		if decision.Changed {
			if err := persistReviewUsage(tx, reservation.Identity, decision.Attempt.ReviewUsage); err != nil {
				return err
			}
		}
		result, acquired = decision.Attempt, decision.Acquired
		return nil
	})
	return result, acquired, err
}

func (r *gormRepository) SettleSlotReviewV3(ctx context.Context, reservation imageagent.SlotReviewUsageReservation, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormReview(ctx, reservation, receipt, imageagent.SlotBudgetCommitted)
}

func (r *gormRepository) ReleaseSlotReviewBudgetV3(ctx context.Context, reservation imageagent.SlotReviewUsageReservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormReview(ctx, reservation, imageagent.SlotUsageReceipt{}, imageagent.SlotBudgetReleased)
}

func (r *gormRepository) MarkSlotReviewBudgetUnknownV3(ctx context.Context, reservation imageagent.SlotReviewUsageReservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormReview(ctx, reservation, imageagent.SlotUsageReceipt{}, imageagent.SlotBudgetUnknown)
}

func (r *gormRepository) transitionGormReview(ctx context.Context, reservation imageagent.SlotReviewUsageReservation, receipt imageagent.SlotUsageReceipt, target imageagent.SlotBudgetStatus) (imageagent.SlotEffectV3Attempt, error) {
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
		var decision effectpolicy.AccountingDecision
		switch target {
		case imageagent.SlotBudgetCommitted:
			decision, err = effectpolicy.SettleReview(effect, reservation, receipt, accounting, time.Now().UTC())
		case imageagent.SlotBudgetReleased:
			decision, err = effectpolicy.ReleaseReview(effect, reservation, accounting)
		case imageagent.SlotBudgetUnknown:
			decision, err = effectpolicy.MarkReviewUnknown(effect, reservation, accounting)
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
			if err := persistReviewUsage(tx, reservation.Identity, decision.Attempt.ReviewUsage); err != nil {
				return err
			}
		}
		result = decision.Attempt
		return nil
	})
	return result, err
}

func persistReviewUsage(tx *gorm.DB, identity imageagent.SlotExternalEffectIdentity, usage []imageagent.SlotReviewUsageAttempt) error {
	encoded, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	return slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), identity).Updates(map[string]any{"review_usage_json": encoded}).Error
}
