package store

import (
	"bytes"
	"context"
	"encoding/json"
	"task-processor/internal/imageagent"

	"gorm.io/gorm"
)

func (r *gormRepository) PrepareGenerationIntent(ctx context.Context, intent imageagent.GenerationIntent) (imageagent.GenerationFact, error) {
	initial, err := imageagent.NewGenerationFact(intent)
	if err != nil {
		return initial, err
	}
	return r.mutateGenerationFact(ctx, intent, false, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		if existing != nil {
			return *existing, nil
		}
		return initial, nil
	})
}
func (r *gormRepository) ReadGenerationFact(ctx context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error) {
	if r.scopeProtocol != imageagent.OrganizationScopeProtocol {
		return imageagent.GenerationFact{}, imageagent.ErrIdentityRequired
	}
	row, err := findSlotEffectV3(ctx, r.db, identity)
	if err != nil {
		return imageagent.GenerationFact{}, err
	}
	fact, err := decodeGenerationFact(row.GenerationFactJSON)
	if err != nil {
		return imageagent.GenerationFact{}, err
	}
	if fact.Intent.Identity != identity {
		return imageagent.GenerationFact{}, imageagent.ErrRevisionConflict
	}
	var run runRecord
	if err := runScopeWhere(r.db.WithContext(ctx), identity.RunScope).Take(&run).Error; err != nil {
		return imageagent.GenerationFact{}, err
	}
	if run.ScopeProtocol != imageagent.OrganizationScopeProtocol || run.MemberID != fact.Intent.MemberID {
		return imageagent.GenerationFact{}, imageagent.ErrIdentityRequired
	}
	return fact, nil
}
func (r *gormRepository) BindGenerationReservation(ctx context.Context, intent imageagent.GenerationIntent, receipt imageagent.GenerationReservationReceipt) (imageagent.GenerationFact, error) {
	return r.mutateGenerationFact(ctx, intent, true, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		return existing.BindReservation(receipt)
	})
}
func (r *gormRepository) BeginGenerationDispatch(ctx context.Context, intent imageagent.GenerationIntent) (imageagent.GenerationFact, bool, error) {
	won := false
	// Deliberately no transaction retry or readback here: an ambiguous COMMIT
	// must never be converted into permission to send the external POST.
	fact, err := r.mutateGenerationFact(ctx, intent, true, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		next, acquired, err := existing.BeginDispatch()
		won = acquired
		return next, err
	})
	if err != nil {
		return imageagent.GenerationFact{}, false, err
	}
	return fact, won, nil
}
func (r *gormRepository) RecordGenerationNoEffect(ctx context.Context, intent imageagent.GenerationIntent) (imageagent.GenerationFact, error) {
	return r.mutateGenerationFact(ctx, intent, true, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		return existing.RecordNoGeneration()
	})
}
func (r *gormRepository) MarkGenerationUnknown(ctx context.Context, intent imageagent.GenerationIntent) (imageagent.GenerationFact, error) {
	return r.mutateGenerationFact(ctx, intent, true, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		return existing.MarkUnknown()
	})
}
func (r *gormRepository) RecordGenerationSuccess(ctx context.Context, intent imageagent.GenerationIntent, proof imageagent.GenerationSuccess) (imageagent.GenerationFact, error) {
	return r.mutateGenerationFact(ctx, intent, true, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		return existing.RecordSuccess(proof)
	})
}

func (r *gormRepository) BindGenerationSettlement(ctx context.Context, intent imageagent.GenerationIntent, receipt imageagent.GenerationSettlementReceipt) (imageagent.GenerationFact, error) {
	return r.mutateGenerationFact(ctx, intent, true, func(existing *imageagent.GenerationFact) (imageagent.GenerationFact, error) {
		return existing.BindSettlement(receipt)
	})
}

func (r *gormRepository) mutateGenerationFact(ctx context.Context, intent imageagent.GenerationIntent, requireExisting bool, change func(*imageagent.GenerationFact) (imageagent.GenerationFact, error)) (imageagent.GenerationFact, error) {
	expected, err := imageagent.NewGenerationFact(intent)
	if err != nil {
		return expected, err
	}
	if r.scopeProtocol != imageagent.OrganizationScopeProtocol {
		return imageagent.GenerationFact{}, imageagent.ErrIdentityRequired
	}
	var result imageagent.GenerationFact
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := r.findRunForUpdate(ctx, tx, intent.Identity.RunScope)
		if err != nil {
			return err
		}
		if run.ScopeProtocol != imageagent.OrganizationScopeProtocol || run.MemberID != intent.MemberID {
			return imageagent.ErrIdentityRequired
		}
		row, err := r.findSlotEffectV3ForUpdate(ctx, tx, intent.Identity)
		if err != nil {
			return err
		}
		var prior *imageagent.GenerationFact
		if len(row.GenerationFactJSON) > 0 {
			stored, err := decodeGenerationFact(row.GenerationFactJSON)
			if err != nil {
				return err
			}
			if stored.IntentID != expected.IntentID || stored.Fingerprint != expected.Fingerprint {
				return imageagent.ErrRevisionConflict
			}
			prior = &stored
		} else if requireExisting {
			return imageagent.ErrRunNotFound
		} else if row.Phase != string(imageagent.SlotEffectV3ProviderClaimed) {
			return imageagent.ErrRevisionConflict
		} else {
			var catalog assetCatalogManifestRecord
			if err := tx.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", intent.Identity.TenantID, intent.Identity.OwnerUserID, intent.Identity.RunID).Take(&catalog).Error; err != nil {
				return err
			}
			if catalog.Hash != intent.CatalogHash {
				return imageagent.ErrRevisionConflict
			}
		}
		next, err := change(prior)
		if err != nil {
			return err
		}
		if err := next.Validate(); err != nil {
			return err
		}
		encoded, err := json.Marshal(next)
		if err != nil {
			return err
		}
		if !bytes.Equal(encoded, row.GenerationFactJSON) {
			updated := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), intent.Identity).Update("generation_fact_json", encoded)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return imageagent.ErrRevisionConflict
			}
		}
		result = next
		return nil
	})
	if err != nil {
		return imageagent.GenerationFact{}, err
	}
	return result, nil
}

func decodeGenerationFact(encoded []byte) (imageagent.GenerationFact, error) {
	var fact imageagent.GenerationFact
	if len(encoded) == 0 {
		return fact, imageagent.ErrRunNotFound
	}
	if err := json.Unmarshal(encoded, &fact); err != nil {
		return fact, imageagent.ErrCorruptPersistedEffect
	}
	if err := fact.Validate(); err != nil {
		return fact, imageagent.ErrCorruptPersistedEffect
	}
	return fact, nil
}
