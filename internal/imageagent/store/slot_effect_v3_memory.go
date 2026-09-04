package store

import (
	"context"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
)

func (r *memoryRepository) ReserveSlotProviderV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
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
	if err := effectpolicy.PreflightBlock(transition); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(transition.Reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.Block(existing, transition)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(decision.Attempt.Identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}

func (r *memoryRepository) RestoreRecoveryBlockedEffectV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := slotEffectKey(reservation.Identity)
	effect, ok := r.slotEffectsV3[key]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	decision, err := effectpolicy.RestoreRecoveryBlocked(effect, reservation)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.slotEffectsV3[key] = cloneSlotEffectV3(decision.Attempt)
	return cloneSlotEffectV3(decision.Attempt), nil
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

func (r *memoryRepository) ResumeReviewRetrySlotV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	if err := effectpolicy.PreflightReservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	effect, ok := r.slotEffectsV3[slotEffectKey(reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	decision, err := effectpolicy.ResumeReviewRetry(effect, reservation)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.slotEffectsV3[slotEffectKey(reservation.Identity)] = cloneSlotEffectV3(decision.Attempt)
	return cloneSlotEffectV3(decision.Attempt), nil
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
	decision, err := effectpolicy.FailClosedCorrupt(identity, effect.CorruptionMarker, &effect)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if decision.Changed {
		r.slotEffectsV3[slotEffectKey(identity)] = cloneSlotEffectV3(decision.Attempt)
	}
	return cloneSlotEffectV3(decision.Attempt), nil
}
