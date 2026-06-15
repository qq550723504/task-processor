package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/tenantctx"
)

func (r *taskRepository) GetCanonicalProductCache(ctx context.Context, fingerprint string) (*canonical.Product, error) {
	if fingerprint == "" {
		return nil, nil
	}
	var entry listingkit.CanonicalProductCacheEntry
	db := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id")
	if err := db.Where("fingerprint = ?", storedCanonicalFingerprint(ctx, fingerprint)).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return entry.CanonicalProduct()
}

func (r *taskRepository) SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *canonical.Product, sourceTaskID string) error {
	entry, err := listingkit.NewCanonicalProductCacheEntry(fingerprint, product, sourceTaskID)
	if err != nil {
		return err
	}
	entry.TenantID = tenantctx.TenantIDFromContext(ctx)
	entry.Fingerprint = storedCanonicalFingerprint(ctx, fingerprint)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "fingerprint"}},
			DoUpdates: clause.Assignments(map[string]any{
				"product":        entry.Product,
				"tenant_id":      entry.TenantID,
				"source_task_id": sourceTaskID,
				"updated_at":     currentTimestampValue(r.db),
			}),
		}).
		Create(entry).Error
}

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

func storedCanonicalFingerprint(ctx context.Context, fingerprint string) string {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	if tenantID == tenantctx.DefaultTenantID {
		return fingerprint
	}
	return tenantID + ":" + fingerprint
}
