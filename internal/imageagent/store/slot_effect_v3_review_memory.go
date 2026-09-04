package store

import (
	"context"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
)

func (r *memoryRepository) ReserveSlotReviewV3(_ context.Context, reservation imageagent.SlotReviewUsageReservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	effect, ok := r.slotEffectsV3[slotEffectKey(reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, false, imageagent.ErrRunNotFound
	}
	runKey := scopeKey(reservation.Identity.RunScope)
	accounting, err := providerAccountingSnapshot(r.runs[runKey], r.reservedUsage[runKey])
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	decision, err := effectpolicy.ReserveReview(&effect, reservation, accounting)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	if err := r.persistMemoryProviderDecision(runKey, slotEffectKey(reservation.Identity), decision.AccountingDecision); err != nil {
		return imageagent.SlotEffectV3Attempt{}, false, err
	}
	return cloneSlotEffectV3(decision.Attempt), decision.Acquired, nil
}

func (r *memoryRepository) SettleSlotReviewV3(_ context.Context, reservation imageagent.SlotReviewUsageReservation, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemoryReview(reservation, receipt, imageagent.SlotBudgetCommitted)
}

func (r *memoryRepository) ReleaseSlotReviewBudgetV3(_ context.Context, reservation imageagent.SlotReviewUsageReservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemoryReview(reservation, imageagent.SlotUsageReceipt{}, imageagent.SlotBudgetReleased)
}

func (r *memoryRepository) MarkSlotReviewBudgetUnknownV3(_ context.Context, reservation imageagent.SlotReviewUsageReservation) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemoryReview(reservation, imageagent.SlotUsageReceipt{}, imageagent.SlotBudgetUnknown)
}

func (r *memoryRepository) transitionMemoryReview(reservation imageagent.SlotReviewUsageReservation, receipt imageagent.SlotUsageReceipt, target imageagent.SlotBudgetStatus) (imageagent.SlotEffectV3Attempt, error) {
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
		decision, err = effectpolicy.SettleReview(effect, reservation, receipt, accounting, r.clock().UTC())
	case imageagent.SlotBudgetReleased:
		decision, err = effectpolicy.ReleaseReview(effect, reservation, accounting)
	case imageagent.SlotBudgetUnknown:
		decision, err = effectpolicy.MarkReviewUnknown(effect, reservation, accounting)
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
