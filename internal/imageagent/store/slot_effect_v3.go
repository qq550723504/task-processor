package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/imageagent"
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
		if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		if !sameSlotEffectV3Reservation(existing, reservation) {
			return imageagent.SlotEffectV3Attempt{}, false, imageagent.ErrRevisionConflict
		}
		return cloneSlotEffectV3(existing), false, nil
	}
	for _, existing := range r.slotEffectsV3 {
		if existing.Identity.RunScope == reservation.Identity.RunScope && existing.IdempotencyKey == reservation.IdempotencyKey {
			if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
				return imageagent.SlotEffectV3Attempt{}, false, err
			}
			return imageagent.SlotEffectV3Attempt{}, false, imageagent.ErrRevisionConflict
		}
	}
	created := imageagent.SlotEffectV3Attempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: imageagent.SlotEffectV3ProviderClaimed, Policy: reservation.Policy, Quote: cloneSlotUsageQuote(reservation.Quote)}
	if reservation.Quote.Fingerprint != "" {
		run := r.runs[runKey]
		persistedPolicy, err := run.Budget.Policy()
		if err != nil || persistedPolicy != reservation.Policy {
			return imageagent.SlotEffectV3Attempt{}, false, imageagent.ErrRevisionConflict
		}
		committed, err := imageagent.UsageVectorFromBudgetUsage(run.Usage)
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		if err := persistedPolicy.Allows(committed, r.reservedUsage[runKey], reservation.Quote.Maximum); err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		reserved, err := imageagent.CheckedAddUsage(r.reservedUsage[runKey], reservation.Quote.Maximum)
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, false, err
		}
		r.reservedUsage[runKey] = reserved
		created.BudgetStatus = imageagent.SlotBudgetReserved
	}
	r.slotEffectsV3[key] = cloneSlotEffectV3(created)
	return cloneSlotEffectV3(created), true, nil
}

func (r *memoryRepository) SettleSlotProviderV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionMemorySlotBudget(reservation, imageagent.SlotBudgetCommitted, receipt)
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
	if !sameSlotEffectV3Reservation(effect, reservation) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if effect.BudgetStatus == target {
		if target != imageagent.SlotBudgetCommitted || reflect.DeepEqual(effect.Receipt, receipt) {
			return cloneSlotEffectV3(effect), nil
		}
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if effect.BudgetStatus != imageagent.SlotBudgetReserved {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if target == imageagent.SlotBudgetCommitted {
		if err := validateUsageReceipt(effect.Quote, receipt); err != nil {
			return imageagent.SlotEffectV3Attempt{}, err
		}
	}
	runKey := scopeKey(reservation.Identity.RunScope)
	reserved, err := imageagent.CheckedSubtractUsage(r.reservedUsage[runKey], effect.Quote.Maximum)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if target != imageagent.SlotBudgetUnknown {
		r.reservedUsage[runKey] = reserved
	}
	if target == imageagent.SlotBudgetCommitted {
		run := r.runs[runKey]
		committed, err := imageagent.UsageVectorFromBudgetUsage(run.Usage)
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, err
		}
		committed, err = imageagent.CheckedAddUsage(committed, receipt.Actual)
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, err
		}
		run.Usage, err = imageagent.BudgetUsageFromUsageVector(committed, run.Usage.Elapsed)
		if err != nil {
			return imageagent.SlotEffectV3Attempt{}, err
		}
		r.runs[runKey] = run
		effect.Receipt = cloneSlotUsageReceipt(receipt)
	}
	effect.BudgetStatus = target
	r.slotEffectsV3[key] = cloneSlotEffectV3(effect)
	return cloneSlotEffectV3(effect), nil
}

func (r *memoryRepository) PrepareSlotStagingV3(_ context.Context, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	fingerprint, err := imageagent.StagingManifestFingerprint(manifest)
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
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if !sameSlotEffectV3Reservation(existing, reservation) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase == imageagent.SlotEffectV3StagingPrepared && existing.StagingManifestFingerprint == fingerprint {
		return cloneSlotEffectV3(existing), nil
	}
	if existing.Phase != imageagent.SlotEffectV3ProviderClaimed {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	existing.Phase = imageagent.SlotEffectV3StagingPrepared
	existing.StagingManifest = cloneStagingManifest(manifest)
	existing.StagingManifestFingerprint = fingerprint
	r.slotEffectsV3[slotEffectKey(reservation.Identity)] = cloneSlotEffectV3(existing)
	return cloneSlotEffectV3(existing), nil
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
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if !sameSlotEffectV3Reservation(existing, reservation) || existing.StagingManifestFingerprint != fingerprint {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase == imageagent.SlotEffectV3ArtifactStaged {
		return cloneSlotEffectV3(existing), nil
	}
	if existing.Phase != imageagent.SlotEffectV3StagingPrepared {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	existing.Phase = imageagent.SlotEffectV3ArtifactStaged
	r.slotEffectsV3[slotEffectKey(reservation.Identity)] = cloneSlotEffectV3(existing)
	return cloneSlotEffectV3(existing), nil
}

func (r *memoryRepository) ClaimSlotPublicationV3(_ context.Context, request imageagent.PublicationClaimRequest) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, error) {
	finalManifest, err := imageagent.NormalizeFinalManifest(request.FinalManifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	request.FinalManifest = finalManifest
	if err := validatePublicationClaimRequestV3(request); err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(request.Reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, imageagent.ErrRunNotFound
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	if !sameSlotEffectV3Reservation(existing, request.Reservation) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, imageagent.ErrRevisionConflict
	}
	return claimSlotPublicationV3(existing, request, r.clock().UTC(), func(updated imageagent.SlotEffectV3Attempt) {
		r.slotEffectsV3[slotEffectKey(updated.Identity)] = cloneSlotEffectV3(updated)
	})
}

func (r *memoryRepository) RenewSlotPublicationV3(_ context.Context, renewal imageagent.PublicationLeaseRenewal) (imageagent.PublicationClaim, error) {
	if err := validatePublicationLeaseRenewalV3(renewal); err != nil {
		return imageagent.PublicationClaim{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(renewal.Identity)]
	if !ok {
		return imageagent.PublicationClaim{}, imageagent.ErrRunNotFound
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
		return imageagent.PublicationClaim{}, err
	}
	return renewSlotPublicationV3(existing, renewal, r.clock().UTC(), func(updated imageagent.SlotEffectV3Attempt) {
		r.slotEffectsV3[slotEffectKey(updated.Identity)] = cloneSlotEffectV3(updated)
	})
}

func (r *memoryRepository) CompleteSlotPublicationV3(_ context.Context, completion imageagent.PublicationCompletion) (imageagent.SlotEffectV3Attempt, error) {
	published, err := imageagent.NormalizeSlotEffectV3PublishedResult(completion.Published)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	completion.Published = published
	if err := validatePublicationCompletionV3(completion); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.slotEffectsV3[slotEffectKey(completion.Reservation.Identity)]
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(existing); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if !sameSlotEffectV3Reservation(existing, completion.Reservation) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase == imageagent.SlotEffectV3PublicationComplete {
		if samePublicationCompletionV3(existing, completion) {
			return cloneSlotEffectV3(existing), nil
		}
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase != imageagent.SlotEffectV3PublicationClaimed || existing.Publication.Owner != completion.Owner || existing.Publication.Fence != completion.Fence || existing.PublicationFingerprint != completion.PublicationFingerprint {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if err := imageagent.ValidateSlotEffectV3Completion(completion.Published, existing.FinalManifest, completion.ResultFingerprint); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	existing.Phase = imageagent.SlotEffectV3PublicationComplete
	existing.ResultFingerprint = completion.ResultFingerprint
	existing.Published = cloneSlotEffectV3PublishedResult(completion.Published)
	r.slotEffectsV3[slotEffectKey(existing.Identity)] = cloneSlotEffectV3(existing)
	return cloneSlotEffectV3(existing), nil
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
	if isBlockedV3Phase(existing.Phase) || !canBlockV3(existing.Phase, transition.Phase) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	if transition.Phase == imageagent.SlotEffectV3PublicationUnknown && (existing.Publication.Owner != transition.Owner || existing.Publication.Fence != transition.Fence) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	existing.Phase = transition.Phase
	existing.BlockedCode = transition.Code
	r.slotEffectsV3[slotEffectKey(existing.Identity)] = cloneSlotEffectV3(existing)
	return cloneSlotEffectV3(existing), nil
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
		if existingRow, findErr := findSlotEffectV3ForUpdate(ctx, tx, reservation.Identity); findErr == nil {
			existing, decodeErr := decodeSlotEffectV3Record(existingRow)
			if decodeErr != nil {
				return decodeErr
			}
			if !sameSlotEffectV3Reservation(existing, reservation) {
				return imageagent.ErrRevisionConflict
			}
			result = existing
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
		var nextReserved imageagent.UsageVector
		if reservation.Quote.Fingerprint != "" {
			run, decodeErr := recordToRun(lockedRun)
			if decodeErr != nil {
				return decodeErr
			}
			persistedPolicy, policyErr := run.Budget.Policy()
			if policyErr != nil || persistedPolicy != reservation.Policy {
				return imageagent.ErrRevisionConflict
			}
			committed, usageErr := imageagent.UsageVectorFromBudgetUsage(run.Usage)
			if usageErr != nil {
				return usageErr
			}
			reserved, usageErr := decodeReservedUsage(lockedRun.ReservedUsageJSON)
			if usageErr != nil {
				return usageErr
			}
			if usageErr = persistedPolicy.Allows(committed, reserved, reservation.Quote.Maximum); usageErr != nil {
				return usageErr
			}
			nextReserved, usageErr = imageagent.CheckedAddUsage(reserved, reservation.Quote.Maximum)
			if usageErr != nil {
				return usageErr
			}
		}
		now := time.Now().UTC()
		row := slotEffectV3RecordFromReservation(reservation, now)
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			if reservation.Quote.Fingerprint != "" {
				encoded, marshalErr := json.Marshal(nextReserved)
				if marshalErr != nil {
					return marshalErr
				}
				if updateErr := runScopeWhere(tx.Model(&runRecord{}), reservation.Identity.RunScope).Update("reserved_usage_json", encoded).Error; updateErr != nil {
					return updateErr
				}
			}
			claimed = true
			result, err = decodeSlotEffectV3Record(row)
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
		if !sameSlotEffectV3Reservation(result, reservation) {
			return imageagent.ErrRevisionConflict
		}
		return nil
	})
	return result, claimed, err
}

func (r *gormRepository) SettleSlotProviderV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, receipt imageagent.SlotUsageReceipt) (imageagent.SlotEffectV3Attempt, error) {
	return r.transitionGormSlotBudget(ctx, reservation, imageagent.SlotBudgetCommitted, receipt)
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
		if !sameSlotEffectV3Reservation(effect, reservation) {
			return imageagent.ErrRevisionConflict
		}
		if effect.BudgetStatus == target {
			if target == imageagent.SlotBudgetCommitted && !reflect.DeepEqual(effect.Receipt, receipt) {
				return imageagent.ErrRevisionConflict
			}
			result = effect
			return nil
		}
		if effect.BudgetStatus != imageagent.SlotBudgetReserved {
			return imageagent.ErrRevisionConflict
		}
		if target == imageagent.SlotBudgetCommitted {
			if err := validateUsageReceipt(effect.Quote, receipt); err != nil {
				return err
			}
		}
		updates := map[string]any{"budget_status": string(target)}
		now := time.Now().UTC()
		switch target {
		case imageagent.SlotBudgetCommitted:
			receiptJSON, marshalErr := json.Marshal(receipt)
			if marshalErr != nil {
				return marshalErr
			}
			updates["usage_receipt_json"] = receiptJSON
			updates["budget_settled_at"] = now
		case imageagent.SlotBudgetReleased:
			updates["budget_released_at"] = now
		case imageagent.SlotBudgetUnknown:
			updates["budget_unknown_at"] = now
		default:
			return imageagent.ErrValidation
		}
		if target != imageagent.SlotBudgetUnknown {
			reserved, decodeErr := decodeReservedUsage(runRow.ReservedUsageJSON)
			if decodeErr != nil {
				return decodeErr
			}
			reserved, decodeErr = imageagent.CheckedSubtractUsage(reserved, effect.Quote.Maximum)
			if decodeErr != nil {
				return decodeErr
			}
			runUpdates := map[string]any{}
			reservedJSON, marshalErr := json.Marshal(reserved)
			if marshalErr != nil {
				return marshalErr
			}
			runUpdates["reserved_usage_json"] = reservedJSON
			if target == imageagent.SlotBudgetCommitted {
				run, decodeRunErr := recordToRun(runRow)
				if decodeRunErr != nil {
					return decodeRunErr
				}
				committed, usageErr := imageagent.UsageVectorFromBudgetUsage(run.Usage)
				if usageErr != nil {
					return usageErr
				}
				committed, usageErr = imageagent.CheckedAddUsage(committed, receipt.Actual)
				if usageErr != nil {
					return usageErr
				}
				run.Usage, usageErr = imageagent.BudgetUsageFromUsageVector(committed, run.Usage.Elapsed)
				if usageErr != nil {
					return usageErr
				}
				usageJSON, marshalErr := json.Marshal(run.Usage)
				if marshalErr != nil {
					return marshalErr
				}
				runUpdates["usage_json"] = usageJSON
			}
			if updateErr := runScopeWhere(tx.Model(&runRecord{}), reservation.Identity.RunScope).Updates(runUpdates).Error; updateErr != nil {
				return updateErr
			}
		}
		if updateErr := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Updates(updates).Error; updateErr != nil {
			return updateErr
		}
		effect.BudgetStatus = target
		if target == imageagent.SlotBudgetCommitted {
			effect.Receipt = cloneSlotUsageReceipt(receipt)
		}
		result = effect
		return nil
	})
	return result, err
}

func (r *gormRepository) PrepareSlotStagingV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	fingerprint, err := imageagent.StagingManifestFingerprint(manifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	if err := validateSlotEffectV3Reservation(reservation); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
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
		if !sameSlotEffectV3Reservation(current, reservation) {
			return imageagent.ErrRevisionConflict
		}
		if current.Phase == imageagent.SlotEffectV3StagingPrepared && current.StagingManifestFingerprint == fingerprint {
			result = current
			return nil
		}
		if current.Phase != imageagent.SlotEffectV3ProviderClaimed {
			return imageagent.ErrRevisionConflict
		}
		now := time.Now().UTC()
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Where("phase = ?", string(imageagent.SlotEffectV3ProviderClaimed)).Updates(map[string]any{"phase": string(imageagent.SlotEffectV3StagingPrepared), "staging_manifest_json": encoded, "staging_manifest_fingerprint": fingerprint, "staging_prepared_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Phase = imageagent.SlotEffectV3StagingPrepared
		current.StagingManifest = cloneStagingManifest(manifest)
		current.StagingManifestFingerprint = fingerprint
		result = current
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
		if !sameSlotEffectV3Reservation(current, reservation) || current.StagingManifestFingerprint != fingerprint {
			return imageagent.ErrRevisionConflict
		}
		if current.Phase == imageagent.SlotEffectV3ArtifactStaged {
			result = current
			return nil
		}
		if current.Phase != imageagent.SlotEffectV3StagingPrepared {
			return imageagent.ErrRevisionConflict
		}
		now := time.Now().UTC()
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), reservation.Identity).Where("phase = ?", string(imageagent.SlotEffectV3StagingPrepared)).Updates(map[string]any{"phase": string(imageagent.SlotEffectV3ArtifactStaged), "staged_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Phase = imageagent.SlotEffectV3ArtifactStaged
		result = current
		return nil
	})
	return result, err
}

func (r *gormRepository) ClaimSlotPublicationV3(ctx context.Context, request imageagent.PublicationClaimRequest) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, error) {
	finalManifest, err := imageagent.NormalizeFinalManifest(request.FinalManifest)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	request.FinalManifest = finalManifest
	if err := validatePublicationClaimRequestV3(request); err != nil {
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
		if !sameSlotEffectV3Reservation(current, request.Reservation) {
			return imageagent.ErrRevisionConflict
		}
		var update *imageagent.SlotEffectV3Attempt
		result, claim, claimed, update, err = evaluatePublicationClaimV3(current, request, now)
		if err != nil {
			return err
		}
		if update == nil {
			return nil
		}
		finalJSON, err := json.Marshal(update.FinalManifest)
		if err != nil {
			return err
		}
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), request.Reservation.Identity).Updates(map[string]any{"phase": string(update.Phase), "publication_owner": update.Publication.Owner, "publication_lease_expires_at": update.Publication.LeaseExpiresAt, "publication_fence": update.Publication.Fence, "publication_fingerprint": update.PublicationFingerprint, "final_manifest_json": finalJSON})
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
	if err := validatePublicationLeaseRenewalV3(renewal); err != nil {
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
		if current.Phase != imageagent.SlotEffectV3PublicationClaimed || current.Publication.Owner != renewal.Owner || current.Publication.Fence != renewal.Fence || !now.Before(current.Publication.LeaseExpiresAt) {
			return imageagent.ErrRevisionConflict
		}
		claim = current.Publication
		claim.LeaseExpiresAt = now.Add(renewal.LeaseDuration)
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
	published, err := imageagent.NormalizeSlotEffectV3PublishedResult(completion.Published)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	completion.Published = published
	if err := validatePublicationCompletionV3(completion); err != nil {
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
		if !sameSlotEffectV3Reservation(current, completion.Reservation) {
			return imageagent.ErrRevisionConflict
		}
		if current.Phase == imageagent.SlotEffectV3PublicationComplete {
			if samePublicationCompletionV3(current, completion) {
				result = current
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
		if current.Phase != imageagent.SlotEffectV3PublicationClaimed || current.Publication.Owner != completion.Owner || current.Publication.Fence != completion.Fence || current.PublicationFingerprint != completion.PublicationFingerprint {
			return imageagent.ErrRevisionConflict
		}
		if err := imageagent.ValidateSlotEffectV3Completion(completion.Published, current.FinalManifest, completion.ResultFingerprint); err != nil {
			return err
		}
		now := time.Now().UTC()
		updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), completion.Reservation.Identity).Where("phase = ? AND publication_owner = ? AND publication_fence = ?", string(imageagent.SlotEffectV3PublicationClaimed), completion.Owner, completion.Fence).Updates(map[string]any{"phase": string(imageagent.SlotEffectV3PublicationComplete), "result_fingerprint": completion.ResultFingerprint, "published_json": encoded, "published_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Phase = imageagent.SlotEffectV3PublicationComplete
		current.ResultFingerprint = completion.ResultFingerprint
		current.Published = cloneSlotEffectV3PublishedResult(completion.Published)
		result = current
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
		if isBlockedV3Phase(current.Phase) || !canBlockV3(current.Phase, transition.Phase) {
			return imageagent.ErrRevisionConflict
		}
		if transition.Phase == imageagent.SlotEffectV3PublicationUnknown && (current.Publication.Owner != transition.Owner || current.Publication.Fence != transition.Fence) {
			return imageagent.ErrRevisionConflict
		}
		updates := map[string]any{"phase": string(transition.Phase), "blocked_code": transition.Code}
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

func claimSlotPublicationV3(current imageagent.SlotEffectV3Attempt, request imageagent.PublicationClaimRequest, now time.Time, persist func(imageagent.SlotEffectV3Attempt)) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, error) {
	result, claim, claimed, update, err := evaluatePublicationClaimV3(current, request, now)
	if err != nil {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, err
	}
	if update != nil {
		persist(*update)
	}
	return result, claim, claimed, nil
}

func evaluatePublicationClaimV3(current imageagent.SlotEffectV3Attempt, request imageagent.PublicationClaimRequest, now time.Time) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, *imageagent.SlotEffectV3Attempt, error) {
	if current.Phase == imageagent.SlotEffectV3ArtifactStaged {
		current.Phase = imageagent.SlotEffectV3PublicationClaimed
		current.Publication = imageagent.PublicationClaim{Owner: request.Owner, LeaseExpiresAt: now.Add(request.LeaseDuration), Fence: 1}
		current.PublicationFingerprint = request.PublicationFingerprint
		current.FinalManifest = cloneFinalManifest(request.FinalManifest)
		result := cloneSlotEffectV3(current)
		return result, result.Publication, true, &current, nil
	}
	if current.Phase != imageagent.SlotEffectV3PublicationClaimed && current.Phase != imageagent.SlotEffectV3PublicationComplete {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, nil, imageagent.ErrRevisionConflict
	}
	if current.PublicationFingerprint != request.PublicationFingerprint || !reflect.DeepEqual(current.FinalManifest, request.FinalManifest) {
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, nil, imageagent.ErrRevisionConflict
	}
	if current.Phase == imageagent.SlotEffectV3PublicationComplete || now.Before(current.Publication.LeaseExpiresAt) {
		result := cloneSlotEffectV3(current)
		return result, result.Publication, false, nil, nil
	}
	current.Publication.Owner = request.Owner
	current.Publication.Fence++
	current.Publication.LeaseExpiresAt = now.Add(request.LeaseDuration)
	result := cloneSlotEffectV3(current)
	return result, result.Publication, true, &current, nil
}

func renewSlotPublicationV3(current imageagent.SlotEffectV3Attempt, renewal imageagent.PublicationLeaseRenewal, now time.Time, persist func(imageagent.SlotEffectV3Attempt)) (imageagent.PublicationClaim, error) {
	if current.Phase != imageagent.SlotEffectV3PublicationClaimed || current.Publication.Owner != renewal.Owner || current.Publication.Fence != renewal.Fence || !now.Before(current.Publication.LeaseExpiresAt) {
		return imageagent.PublicationClaim{}, imageagent.ErrRevisionConflict
	}
	current.Publication.LeaseExpiresAt = now.Add(renewal.LeaseDuration)
	persist(current)
	return current.Publication, nil
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

func validatePublicationClaimRequestV3(request imageagent.PublicationClaimRequest) error {
	if err := validateSlotEffectV3Reservation(request.Reservation); err != nil {
		return err
	}
	if strings.TrimSpace(request.Owner) == "" || request.LeaseDuration <= 0 || strings.TrimSpace(request.PublicationFingerprint) == "" {
		return imageagent.ErrValidation
	}
	return imageagent.ValidateFinalManifest(request.FinalManifest)
}

func validatePublicationLeaseRenewalV3(renewal imageagent.PublicationLeaseRenewal) error {
	if err := validateSlotEffectIdentity(renewal.Identity); err != nil {
		return err
	}
	if strings.TrimSpace(renewal.Owner) == "" || renewal.Fence <= 0 || renewal.LeaseDuration <= 0 {
		return imageagent.ErrValidation
	}
	return nil
}

func validatePublicationCompletionV3(completion imageagent.PublicationCompletion) error {
	if err := validateSlotEffectV3Reservation(completion.Reservation); err != nil {
		return err
	}
	if strings.TrimSpace(completion.Owner) == "" || completion.Fence <= 0 || strings.TrimSpace(completion.PublicationFingerprint) == "" || strings.TrimSpace(completion.ResultFingerprint) == "" || completion.Published.SlotID != completion.Reservation.Identity.SlotID || completion.Published.Attempt != completion.Reservation.Identity.Attempt || len(completion.Published.Candidates) == 0 {
		return imageagent.ErrValidation
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
	return phase == imageagent.SlotEffectV3ProviderUnknown || phase == imageagent.SlotEffectV3StagingUnknown || phase == imageagent.SlotEffectV3PublicationUnknown
}

func canBlockV3(current, blocked imageagent.SlotEffectV3Phase) bool {
	switch blocked {
	case imageagent.SlotEffectV3ProviderUnknown:
		return current == imageagent.SlotEffectV3ProviderClaimed
	case imageagent.SlotEffectV3StagingUnknown:
		return current == imageagent.SlotEffectV3StagingPrepared
	case imageagent.SlotEffectV3PublicationUnknown:
		return current == imageagent.SlotEffectV3PublicationClaimed
	default:
		return false
	}
}

func sameSlotEffectV3Reservation(effect imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) bool {
	return effect.Identity == reservation.Identity && effect.IdempotencyKey == reservation.IdempotencyKey && effect.InputFingerprint == reservation.InputFingerprint && effect.Quote.Fingerprint == reservation.Quote.Fingerprint
}

func validateUsageReceipt(quote imageagent.SlotUsageQuote, receipt imageagent.SlotUsageReceipt) error {
	if receipt.Actual.Images < 0 || receipt.Actual.AgentSteps < 0 || receipt.Actual.ModelCalls < 0 || receipt.Actual.CostMicros < 0 ||
		receipt.Actual.Images > quote.Maximum.Images || receipt.Actual.AgentSteps > quote.Maximum.AgentSteps || receipt.Actual.ModelCalls > quote.Maximum.ModelCalls || receipt.Actual.CostMicros > quote.Maximum.CostMicros {
		return imageagent.ErrRevisionConflict
	}
	if receipt.CostBasis == "" {
		return imageagent.ErrValidation
	}
	return nil
}

func samePublicationCompletionV3(effect imageagent.SlotEffectV3Attempt, completion imageagent.PublicationCompletion) bool {
	return effect.Publication.Owner == completion.Owner && effect.Publication.Fence == completion.Fence && effect.PublicationFingerprint == completion.PublicationFingerprint && effect.ResultFingerprint == completion.ResultFingerprint && reflect.DeepEqual(effect.Published, completion.Published)
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

func slotEffectV3RecordFromReservation(reservation imageagent.SlotEffectV3Reservation, claimedAt time.Time) slotExternalEffectV3Record {
	identity := reservation.Identity
	policyJSON, _ := json.Marshal(reservation.Policy)
	quoteJSON, _ := json.Marshal(reservation.Quote)
	status := imageagent.SlotBudgetStatus("")
	if reservation.Quote.Fingerprint != "" {
		status = imageagent.SlotBudgetReserved
	}
	return slotExternalEffectV3Record{TenantID: identity.TenantID, OwnerUserID: identity.OwnerUserID, RunID: identity.RunID, PlanRevision: identity.PlanRevision, SlotID: identity.SlotID, Attempt: identity.Attempt, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: string(imageagent.SlotEffectV3ProviderClaimed), BudgetStatus: string(status), BudgetPolicyJSON: policyJSON, UsageQuoteJSON: quoteJSON, UsageQuoteFingerprint: reservation.Quote.Fingerprint, PricingVersion: reservation.Quote.PricingVersion, ProviderClaimedAt: claimedAt}
}

func decodeSlotEffectV3Record(row slotExternalEffectV3Record) (imageagent.SlotEffectV3Attempt, error) {
	result := slotEffectV3FromRecord(row)
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
			return result, fmt.Errorf("decode v3 budget policy: %w", err)
		}
	}
	if len(row.UsageQuoteJSON) > 0 {
		if err := json.Unmarshal(row.UsageQuoteJSON, &result.Quote); err != nil {
			return result, fmt.Errorf("decode v3 usage quote: %w", err)
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

func slotEffectV3FromRecord(row slotExternalEffectV3Record) imageagent.SlotEffectV3Attempt {
	claim := imageagent.PublicationClaim{Owner: row.PublicationOwner, Fence: row.PublicationFence}
	if row.PublicationLeaseExpiresAt != nil {
		claim.LeaseExpiresAt = row.PublicationLeaseExpiresAt.UTC()
	}
	return imageagent.SlotEffectV3Attempt{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: row.TenantID, OwnerUserID: row.OwnerUserID, RunID: row.RunID}, PlanRevision: row.PlanRevision, SlotID: row.SlotID, Attempt: row.Attempt}, IdempotencyKey: row.IdempotencyKey, InputFingerprint: row.InputFingerprint, Phase: imageagent.SlotEffectV3Phase(row.Phase), StagingManifestFingerprint: row.StagingManifestFingerprint, Publication: claim, PublicationFingerprint: row.PublicationFingerprint, ResultFingerprint: row.ResultFingerprint, BlockedCode: row.BlockedCode, BudgetStatus: imageagent.SlotBudgetStatus(row.BudgetStatus)}
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
