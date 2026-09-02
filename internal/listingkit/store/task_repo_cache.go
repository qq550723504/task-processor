package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
)

func (r *taskRepository) GetSDSBaselineCache(ctx context.Context, tenantID, baselineKey string) (*listingkit.SDSBaselineCacheEntry, error) {
	resolvedTenantID, logicalKey, storedKey, err := listingkit.ResolveSDSBaselineCacheScope(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, err
	}
	if storedKey == "" {
		return nil, nil
	}
	var entry listingkit.SDSBaselineCacheEntry
	db := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id")
	if err := db.Where("baseline_key = ?", storedKey).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	entry.TenantID = resolvedTenantID
	entry.BaselineKey = logicalKey
	return &entry, nil
}

func (r *taskRepository) SaveSDSBaselineCache(ctx context.Context, entry *listingkit.SDSBaselineCacheEntry) error {
	if entry == nil {
		return nil
	}
	tenantID, _, storedKey, err := listingkit.ResolveSDSBaselineCacheScope(ctx, entry.TenantID, entry.BaselineKey)
	if err != nil {
		return err
	}
	if storedKey == "" {
		return nil
	}
	cloned, err := entry.Clone()
	if err != nil {
		return err
	}
	cloned.TenantID = tenantID
	cloned.BaselineKey = storedKey
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "baseline_key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"tenant_id":              cloned.TenantID,
				"status":                 cloned.Status,
				"version":                cloned.Version,
				"source_task_id":         cloned.SourceTaskID,
				"identity":               cloned.Identity,
				"canonical_product_base": cloned.CanonicalProductBase,
				"validation_status":      cloned.ValidationStatus,
				"validation_reason_code": cloned.ValidationReasonCode,
				"validation_reason":      cloned.ValidationReason,
				"validated_at":           cloned.ValidatedAt,
				"updated_at":             time.Now(),
			}),
		}).
		Create(cloned).Error
}
