package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sheinpub "task-processor/internal/publishing/shein"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormStoreProfileRepository struct {
	db *gorm.DB
}

type listingKitStoreProfileRecord struct {
	ID                int64 `gorm:"primaryKey"`
	TenantID          int64 `gorm:"index:idx_listingkit_store_profile_tenant_priority,priority:1;uniqueIndex:uk_listingkit_store_profile_tenant_store,priority:1"`
	StoreID           int64 `gorm:"uniqueIndex:uk_listingkit_store_profile_tenant_store,priority:2"`
	Enabled           bool
	Priority          int `gorm:"index:idx_listingkit_store_profile_tenant_priority,priority:2"`
	IsFallback        bool
	Site              string
	WarehouseCode     string
	DefaultStock      int
	DefaultSubmitMode string
	PricingJSON       string `gorm:"column:pricing;type:text"`
	MatchRulesJSON    string `gorm:"column:match_rules;type:text"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (listingKitStoreProfileRecord) TableName() string {
	return "listingkit_store_profiles"
}

func NewGormStoreProfileRepository(db *gorm.DB) StoreProfileRepository {
	return &gormStoreProfileRepository{db: db}
}

func AutoMigrateStoreProfileRepository(db *gorm.DB) error {
	return db.AutoMigrate(&listingKitStoreProfileRecord{}, &listingKitStoreRoutingSettingsRecord{})
}

func (r *gormStoreProfileRepository) ListByTenant(ctx context.Context, tenantID int64) ([]ListingKitStoreProfile, error) {
	var rows []listingKitStoreProfileRecord
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("priority ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ListingKitStoreProfile, 0, len(rows))
	for _, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *gormStoreProfileRepository) Upsert(ctx context.Context, profile *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {
	if profile == nil {
		return nil, fmt.Errorf("store profile is required")
	}
	row, err := newListingKitStoreProfileRecord(*profile)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	if row.ID > 0 {
		result := r.db.WithContext(ctx).
			Model(&listingKitStoreProfileRecord{}).
			Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).
			Updates(map[string]any{
				"store_id":            row.StoreID,
				"enabled":             row.Enabled,
				"priority":            row.Priority,
				"is_fallback":         row.IsFallback,
				"site":                row.Site,
				"warehouse_code":      row.WarehouseCode,
				"default_stock":       row.DefaultStock,
				"default_submit_mode": row.DefaultSubmitMode,
				"pricing":             row.PricingJSON,
				"match_rules":         row.MatchRulesJSON,
				"updated_at":          row.UpdatedAt,
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			return nil, fmt.Errorf("store profile not found")
		}
		return row.toDomainPtr()
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "store_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"enabled":             row.Enabled,
			"priority":            row.Priority,
			"is_fallback":         row.IsFallback,
			"site":                row.Site,
			"warehouse_code":      row.WarehouseCode,
			"default_stock":       row.DefaultStock,
			"default_submit_mode": row.DefaultSubmitMode,
			"pricing":             row.PricingJSON,
			"match_rules":         row.MatchRulesJSON,
			"updated_at":          row.UpdatedAt,
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	var persisted listingKitStoreProfileRecord
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND store_id = ?", row.TenantID, row.StoreID).
		Take(&persisted).Error; err != nil {
		return nil, err
	}
	return persisted.toDomainPtr()
}

func (r *gormStoreProfileRepository) Delete(ctx context.Context, tenantID, id int64) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&listingKitStoreProfileRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("store profile not found")
	}
	return nil
}

func newListingKitStoreProfileRecord(profile ListingKitStoreProfile) (listingKitStoreProfileRecord, error) {
	pricingJSON, err := json.Marshal(profile.Pricing)
	if err != nil {
		return listingKitStoreProfileRecord{}, fmt.Errorf("marshal store pricing: %w", err)
	}
	matchRulesJSON, err := json.Marshal(profile.MatchRules)
	if err != nil {
		return listingKitStoreProfileRecord{}, fmt.Errorf("marshal store match rules: %w", err)
	}
	row := listingKitStoreProfileRecord{
		ID:                profile.ID,
		TenantID:          profile.TenantID,
		StoreID:           profile.StoreID,
		Enabled:           profile.Enabled,
		Priority:          profile.Priority,
		IsFallback:        profile.IsFallback,
		Site:              profile.Site,
		WarehouseCode:     profile.WarehouseCode,
		DefaultStock:      profile.DefaultStock,
		DefaultSubmitMode: profile.DefaultSubmitMode,
		PricingJSON:       string(pricingJSON),
		MatchRulesJSON:    string(matchRulesJSON),
	}
	if profile.UpdatedAt != nil {
		row.UpdatedAt = profile.UpdatedAt.UTC()
	}
	return row, nil
}

func (r listingKitStoreProfileRecord) toDomain() (ListingKitStoreProfile, error) {
	var pricing sheinPricingEnvelope
	if err := pricing.unmarshal(r.PricingJSON); err != nil {
		return ListingKitStoreProfile{}, err
	}
	var matchRules []ListingKitStoreMatchRule
	if err := unmarshalStoreMatchRules(r.MatchRulesJSON, &matchRules); err != nil {
		return ListingKitStoreProfile{}, err
	}
	updatedAt := r.UpdatedAt.UTC()
	return ListingKitStoreProfile{
		ID:                r.ID,
		TenantID:          r.TenantID,
		StoreID:           r.StoreID,
		Enabled:           r.Enabled,
		Priority:          r.Priority,
		IsFallback:        r.IsFallback,
		Site:              r.Site,
		WarehouseCode:     r.WarehouseCode,
		DefaultStock:      r.DefaultStock,
		DefaultSubmitMode: r.DefaultSubmitMode,
		Pricing:           pricing.Rule,
		MatchRules:        matchRules,
		UpdatedAt:         &updatedAt,
	}, nil
}

func (r listingKitStoreProfileRecord) toDomainPtr() (*ListingKitStoreProfile, error) {
	item, err := r.toDomain()
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type sheinPricingEnvelope struct {
	Rule sheinpub.PricingRule
}

func (p *sheinPricingEnvelope) unmarshal(value string) error {
	if value == "" {
		value = "{}"
	}
	var rule sheinpub.PricingRule
	if err := json.Unmarshal([]byte(value), &rule); err != nil {
		return fmt.Errorf("unmarshal store pricing: %w", err)
	}
	p.Rule = rule
	return nil
}

func unmarshalStoreMatchRules(value string, target *[]ListingKitStoreMatchRule) error {
	if value == "" {
		*target = nil
		return nil
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("unmarshal store match rules: %w", err)
	}
	return nil
}
