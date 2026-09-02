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

func (r *memoryRepository) ReserveSlotExternalEffect(_ context.Context, reservation imageagent.SlotExternalEffectReservation) (imageagent.SlotExternalEffectAttempt, bool, error) {
	if err := validateSlotEffectReservation(reservation); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemorySlotEffectScope(r, reservation.Identity); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, false, err
	}
	key := slotEffectKey(reservation.Identity)
	if existing, ok := r.slotEffects[key]; ok {
		if !sameSlotEffectReservation(existing, reservation) {
			return imageagent.SlotExternalEffectAttempt{}, false, imageagent.ErrRevisionConflict
		}
		return cloneSlotEffect(existing), false, nil
	}
	for _, existing := range r.slotEffects {
		if existing.Identity.RunScope == reservation.Identity.RunScope && existing.IdempotencyKey == reservation.IdempotencyKey {
			return imageagent.SlotExternalEffectAttempt{}, false, imageagent.ErrRevisionConflict
		}
	}
	created := imageagent.SlotExternalEffectAttempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: imageagent.SlotExternalEffectProviderStarted}
	r.slotEffects[key] = cloneSlotEffect(created)
	return cloneSlotEffect(created), true, nil
}

func (r *memoryRepository) StoreSlotGeneratedOutput(_ context.Context, reservation imageagent.SlotExternalEffectReservation, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExternalEffectAttempt, error) {
	if err := validateGeneratedTransition(reservation, generated); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := slotEffectKey(reservation.Identity)
	existing, ok := r.slotEffects[key]
	if !ok {
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRunNotFound
	}
	if !sameSlotEffectReservation(existing, reservation) {
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase != imageagent.SlotExternalEffectProviderStarted {
		if reflect.DeepEqual(existing.Generated, generated) {
			return cloneSlotEffect(existing), nil
		}
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRevisionConflict
	}
	existing.Generated = cloneGeneratedOutput(generated)
	existing.Phase = imageagent.SlotExternalEffectGeneratedComplete
	r.slotEffects[key] = existing
	return cloneSlotEffect(existing), nil
}

func (r *memoryRepository) CompleteSlotPublication(_ context.Context, reservation imageagent.SlotExternalEffectReservation, result imageagent.SlotExecutionResult) (imageagent.SlotExternalEffectAttempt, error) {
	if err := validatePublicationTransition(reservation, result); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := slotEffectKey(reservation.Identity)
	existing, ok := r.slotEffects[key]
	if !ok {
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRunNotFound
	}
	if !sameSlotEffectReservation(existing, reservation) || existing.Phase == imageagent.SlotExternalEffectProviderStarted {
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRevisionConflict
	}
	if existing.Phase == imageagent.SlotExternalEffectPublicationComplete {
		if reflect.DeepEqual(existing.Published, result) {
			return cloneSlotEffect(existing), nil
		}
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRevisionConflict
	}
	existing.Published = cloneSlotExecutionResult(result)
	existing.Phase = imageagent.SlotExternalEffectPublicationComplete
	r.slotEffects[key] = existing
	return cloneSlotEffect(existing), nil
}

func (r *memoryRepository) GetSlotExternalEffect(_ context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.SlotExternalEffectAttempt, error) {
	if err := validateSlotEffectIdentity(identity); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(identity.RunScope)]; !ok {
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRunNotFound
	}
	effect, ok := r.slotEffects[slotEffectKey(identity)]
	if !ok {
		return imageagent.SlotExternalEffectAttempt{}, imageagent.ErrRunNotFound
	}
	return cloneSlotEffect(effect), nil
}

func (r *gormRepository) ReserveSlotExternalEffect(ctx context.Context, reservation imageagent.SlotExternalEffectReservation) (imageagent.SlotExternalEffectAttempt, bool, error) {
	if err := validateSlotEffectReservation(reservation); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, false, err
	}
	var result imageagent.SlotExternalEffectAttempt
	claimed := false
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		claimed = false
		if err := validateGormSlotEffectScope(ctx, r, tx, reservation.Identity); err != nil {
			return err
		}
		now := time.Now().UTC()
		row := slotEffectRecordFromReservation(reservation, now)
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			claimed = true
			result = slotEffectFromRecord(row)
			return nil
		}
		existing, err := findSlotEffectForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		result, err = decodeSlotEffectRecord(existing)
		if err != nil {
			return err
		}
		if !sameSlotEffectReservation(result, reservation) {
			return imageagent.ErrRevisionConflict
		}
		return nil
	})
	return result, claimed, err
}

func (r *gormRepository) StoreSlotGeneratedOutput(ctx context.Context, reservation imageagent.SlotExternalEffectReservation, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExternalEffectAttempt, error) {
	if err := validateGeneratedTransition(reservation, generated); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	var result imageagent.SlotExternalEffectAttempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectRecord(row)
		if err != nil {
			return err
		}
		if !sameSlotEffectReservation(current, reservation) {
			return imageagent.ErrRevisionConflict
		}
		if current.Phase != imageagent.SlotExternalEffectProviderStarted {
			if reflect.DeepEqual(current.Generated, generated) {
				result = current
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
		encoded, err := json.Marshal(generated)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		updated := slotEffectIdentityWhere(tx.Model(&slotExternalEffectRecord{}), reservation.Identity).Where("phase = ?", string(imageagent.SlotExternalEffectProviderStarted)).Updates(map[string]any{"phase": string(imageagent.SlotExternalEffectGeneratedComplete), "generated_json": encoded, "generated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Generated = cloneGeneratedOutput(generated)
		current.Phase = imageagent.SlotExternalEffectGeneratedComplete
		result = current
		return nil
	})
	return result, err
}

func (r *gormRepository) CompleteSlotPublication(ctx context.Context, reservation imageagent.SlotExternalEffectReservation, published imageagent.SlotExecutionResult) (imageagent.SlotExternalEffectAttempt, error) {
	if err := validatePublicationTransition(reservation, published); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	var result imageagent.SlotExternalEffectAttempt
	err := withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		row, err := findSlotEffectForUpdate(ctx, tx, reservation.Identity)
		if err != nil {
			return err
		}
		current, err := decodeSlotEffectRecord(row)
		if err != nil {
			return err
		}
		if !sameSlotEffectReservation(current, reservation) || current.Phase == imageagent.SlotExternalEffectProviderStarted {
			return imageagent.ErrRevisionConflict
		}
		if current.Phase == imageagent.SlotExternalEffectPublicationComplete {
			if reflect.DeepEqual(current.Published, published) {
				result = current
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
		encoded, err := json.Marshal(published)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		updated := slotEffectIdentityWhere(tx.Model(&slotExternalEffectRecord{}), reservation.Identity).Where("phase = ?", string(imageagent.SlotExternalEffectGeneratedComplete)).Updates(map[string]any{"phase": string(imageagent.SlotExternalEffectPublicationComplete), "published_json": encoded, "published_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		current.Published = cloneSlotExecutionResult(published)
		current.Phase = imageagent.SlotExternalEffectPublicationComplete
		result = current
		return nil
	})
	return result, err
}

func (r *gormRepository) GetSlotExternalEffect(ctx context.Context, identity imageagent.SlotExternalEffectIdentity) (imageagent.SlotExternalEffectAttempt, error) {
	if err := validateSlotEffectIdentity(identity); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	if _, err := r.findRun(ctx, r.db, identity.RunScope); err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	row, err := findSlotEffect(ctx, r.db, identity)
	if err != nil {
		return imageagent.SlotExternalEffectAttempt{}, err
	}
	return decodeSlotEffectRecord(row)
}

func validateSlotEffectReservation(reservation imageagent.SlotExternalEffectReservation) error {
	if err := validateSlotEffectIdentity(reservation.Identity); err != nil {
		return err
	}
	if strings.TrimSpace(reservation.IdempotencyKey) == "" || strings.TrimSpace(reservation.InputFingerprint) == "" {
		return imageagent.ErrValidation
	}
	return nil
}

func validateSlotEffectIdentity(identity imageagent.SlotExternalEffectIdentity) error {
	if err := validateScope(identity.RunScope); err != nil {
		return err
	}
	if identity.PlanRevision <= 0 || strings.TrimSpace(identity.SlotID) == "" || identity.Attempt <= 0 {
		return imageagent.ErrValidation
	}
	return nil
}

func validateGeneratedTransition(reservation imageagent.SlotExternalEffectReservation, generated imageagent.SlotGeneratedOutput) error {
	if err := validateSlotEffectReservation(reservation); err != nil {
		return err
	}
	if generated.SlotID != reservation.Identity.SlotID || generated.Attempt != reservation.Identity.Attempt || strings.TrimSpace(generated.SourceAssetID) == "" || len(generated.Assets) == 0 {
		return imageagent.ErrRevisionConflict
	}
	return nil
}

func validatePublicationTransition(reservation imageagent.SlotExternalEffectReservation, result imageagent.SlotExecutionResult) error {
	if err := validateSlotEffectReservation(reservation); err != nil {
		return err
	}
	if result.SlotID != reservation.Identity.SlotID || result.Attempt != reservation.Identity.Attempt || len(result.Candidates) == 0 {
		return imageagent.ErrRevisionConflict
	}
	for _, candidate := range result.Candidates {
		if strings.TrimSpace(candidate.AssetID) == "" {
			return imageagent.ErrValidation
		}
		if _, err := imageagent.ValidateSafeImageURL(candidate.URL); err != nil {
			return err
		}
	}
	return nil
}

func validateMemorySlotEffectScope(r *memoryRepository, identity imageagent.SlotExternalEffectIdentity) error {
	run, ok := r.runs[scopeKey(identity.RunScope)]
	if !ok {
		return imageagent.ErrRunNotFound
	}
	if run.ActivePlanRevision != identity.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	if _, ok := r.slots[slotKey(identity.RunScope, identity.PlanRevision, identity.SlotID)]; !ok {
		return imageagent.ErrRevisionConflict
	}
	return nil
}

func validateGormSlotEffectScope(ctx context.Context, repository *gormRepository, tx *gorm.DB, identity imageagent.SlotExternalEffectIdentity) error {
	run, err := repository.findRunForUpdate(ctx, tx, identity.RunScope)
	if err != nil {
		return err
	}
	if run.ActivePlanRevision != identity.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	var count int64
	if err := scopedWhere(tx.Model(&slotRecord{}), identity.RunScope).Where("plan_revision = ? AND id = ?", identity.PlanRevision, identity.SlotID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return imageagent.ErrRevisionConflict
	}
	return nil
}

func findSlotEffectForUpdate(ctx context.Context, db *gorm.DB, identity imageagent.SlotExternalEffectIdentity) (slotExternalEffectRecord, error) {
	return findSlotEffect(ctx, db.Clauses(clause.Locking{Strength: "UPDATE"}), identity)
}

func findSlotEffect(ctx context.Context, db *gorm.DB, identity imageagent.SlotExternalEffectIdentity) (slotExternalEffectRecord, error) {
	var row slotExternalEffectRecord
	err := slotEffectIdentityWhere(db.WithContext(ctx), identity).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, imageagent.ErrRunNotFound
	}
	if err != nil {
		return row, fmt.Errorf("get image agent slot external effect: %w", err)
	}
	return row, nil
}

func slotEffectIdentityWhere(db *gorm.DB, identity imageagent.SlotExternalEffectIdentity) *gorm.DB {
	return db.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND slot_id = ? AND attempt = ?", identity.TenantID, identity.OwnerUserID, identity.RunID, identity.PlanRevision, identity.SlotID, identity.Attempt)
}

func slotEffectRecordFromReservation(reservation imageagent.SlotExternalEffectReservation, startedAt time.Time) slotExternalEffectRecord {
	identity := reservation.Identity
	return slotExternalEffectRecord{TenantID: identity.TenantID, OwnerUserID: identity.OwnerUserID, RunID: identity.RunID, PlanRevision: identity.PlanRevision, SlotID: identity.SlotID, Attempt: identity.Attempt, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: string(imageagent.SlotExternalEffectProviderStarted), ProviderStartedAt: startedAt}
}

func decodeSlotEffectRecord(row slotExternalEffectRecord) (imageagent.SlotExternalEffectAttempt, error) {
	result := slotEffectFromRecord(row)
	if len(row.GeneratedJSON) > 0 {
		if err := json.Unmarshal(row.GeneratedJSON, &result.Generated); err != nil {
			return result, fmt.Errorf("decode generated slot output: %w", err)
		}
	}
	if len(row.PublishedJSON) > 0 {
		if err := json.Unmarshal(row.PublishedJSON, &result.Published); err != nil {
			return result, fmt.Errorf("decode published slot result: %w", err)
		}
	}
	return result, nil
}

func slotEffectFromRecord(row slotExternalEffectRecord) imageagent.SlotExternalEffectAttempt {
	return imageagent.SlotExternalEffectAttempt{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: row.TenantID, OwnerUserID: row.OwnerUserID, RunID: row.RunID}, PlanRevision: row.PlanRevision, SlotID: row.SlotID, Attempt: row.Attempt}, IdempotencyKey: row.IdempotencyKey, InputFingerprint: row.InputFingerprint, Phase: imageagent.SlotExternalEffectPhase(row.Phase)}
}

func sameSlotEffectReservation(effect imageagent.SlotExternalEffectAttempt, reservation imageagent.SlotExternalEffectReservation) bool {
	return effect.Identity == reservation.Identity && effect.IdempotencyKey == reservation.IdempotencyKey && effect.InputFingerprint == reservation.InputFingerprint
}

func slotEffectKey(identity imageagent.SlotExternalEffectIdentity) string {
	return fmt.Sprintf("%s\x00%d\x00%s\x00%d", scopeKey(identity.RunScope), identity.PlanRevision, identity.SlotID, identity.Attempt)
}

func cloneSlotEffect(effect imageagent.SlotExternalEffectAttempt) imageagent.SlotExternalEffectAttempt {
	effect.Generated = cloneGeneratedOutput(effect.Generated)
	effect.Published = cloneSlotExecutionResult(effect.Published)
	return effect
}

func cloneGeneratedOutput(output imageagent.SlotGeneratedOutput) imageagent.SlotGeneratedOutput {
	assets := make([]imageagent.GeneratedAsset, len(output.Assets))
	for index, item := range output.Assets {
		item.Bytes = append([]byte(nil), item.Bytes...)
		item.Operations = append([]string(nil), item.Operations...)
		item.Metadata = cloneMetadata(item.Metadata)
		assets[index] = item
	}
	output.Assets = assets
	return output
}

func cloneSlotExecutionResult(result imageagent.SlotExecutionResult) imageagent.SlotExecutionResult {
	candidates := make([]imageagent.AssetCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		candidate.Metadata = cloneMetadata(candidate.Metadata)
		candidates[index] = candidate
	}
	result.Candidates = candidates
	return result
}

var _ imageagent.SlotExternalEffectRepository = (*memoryRepository)(nil)
var _ imageagent.SlotExternalEffectRepository = (*gormRepository)(nil)
