package listingkit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormStoreRoutingSettingsRepository struct {
	db *gorm.DB
}

type listingKitStoreRoutingSettingsRecord struct {
	TenantID            int64 `gorm:"primaryKey"`
	SelectionStrategy   string
	FallbackStoreID     int64
	AllowManualOverride bool
	AllowFallback       bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (listingKitStoreRoutingSettingsRecord) TableName() string {
	return "listingkit_store_routing_settings"
}

func NewGormStoreRoutingSettingsRepository(db *gorm.DB) StoreRoutingSettingsRepository {
	return &gormStoreRoutingSettingsRepository{db: db}
}

func (r *gormStoreRoutingSettingsRepository) GetByTenant(ctx context.Context, tenantID int64) (*ListingKitStoreRoutingSettings, error) {
	var row listingKitStoreRoutingSettingsRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings := defaultStoreRoutingSettings(tenantID)
			return &settings, nil
		}
		return nil, err
	}
	return row.toDomainPtr(), nil
}

func (r *gormStoreRoutingSettingsRepository) Upsert(ctx context.Context, settings *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	if settings == nil {
		return nil, fmt.Errorf("legacy store routing settings are required")
	}
	row := newListingKitStoreRoutingSettingsRecord(*settings)
	now := time.Now().UTC()
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"selection_strategy":    row.SelectionStrategy,
			"fallback_store_id":     row.FallbackStoreID,
			"allow_manual_override": row.AllowManualOverride,
			"allow_fallback":        row.AllowFallback,
			"updated_at":            row.UpdatedAt,
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	return row.toDomainPtr(), nil
}

func newListingKitStoreRoutingSettingsRecord(settings ListingKitStoreRoutingSettings) listingKitStoreRoutingSettingsRecord {
	return listingKitStoreRoutingSettingsRecord{
		TenantID:            settings.TenantID,
		SelectionStrategy:   settings.SelectionStrategy,
		FallbackStoreID:     settings.FallbackStoreID,
		AllowManualOverride: settings.AllowManualOverride,
		AllowFallback:       settings.AllowFallback,
	}
}

func (r listingKitStoreRoutingSettingsRecord) toDomainPtr() *ListingKitStoreRoutingSettings {
	updatedAt := r.UpdatedAt.UTC()
	return &ListingKitStoreRoutingSettings{
		TenantID:            r.TenantID,
		SelectionStrategy:   r.SelectionStrategy,
		FallbackStoreID:     r.FallbackStoreID,
		AllowManualOverride: r.AllowManualOverride,
		AllowFallback:       r.AllowFallback,
		UpdatedAt:           &updatedAt,
	}
}
