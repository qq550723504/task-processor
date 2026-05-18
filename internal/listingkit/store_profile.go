package listingkit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type ListingKitStoreMatchRule struct {
	Kind   string   `json:"kind,omitempty"`
	Values []string `json:"values,omitempty"`
}

type ListingKitStoreProfile struct {
	ID                int64                      `json:"id"`
	TenantID          int64                      `json:"tenant_id,omitempty"`
	StoreID           int64                      `json:"store_id"`
	Enabled           bool                       `json:"enabled"`
	Priority          int                        `json:"priority,omitempty"`
	IsFallback        bool                       `json:"is_fallback,omitempty"`
	Site              string                     `json:"site,omitempty"`
	WarehouseCode     string                     `json:"warehouse_code,omitempty"`
	DefaultStock      int                        `json:"default_stock,omitempty"`
	DefaultSubmitMode string                     `json:"default_submit_mode,omitempty"`
	Pricing           sheinpub.PricingRule       `json:"pricing,omitempty"`
	MatchRules        []ListingKitStoreMatchRule `json:"match_rules,omitempty"`
	UpdatedAt         *time.Time                 `json:"updated_at,omitempty"`
	Store             *SheinStoreOption          `json:"store,omitempty"`
}

type ListingKitStoreRoutingSettings struct {
	TenantID            int64      `json:"tenant_id,omitempty"`
	SelectionStrategy   string     `json:"selection_strategy,omitempty"`
	FallbackStoreID     int64      `json:"fallback_store_id,omitempty"`
	AllowManualOverride bool       `json:"allow_manual_override,omitempty"`
	AllowFallback       bool       `json:"allow_fallback,omitempty"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

type StoreProfileRepository interface {
	ListByTenant(ctx context.Context, tenantID int64) ([]ListingKitStoreProfile, error)
	Upsert(ctx context.Context, profile *ListingKitStoreProfile) (*ListingKitStoreProfile, error)
	Delete(ctx context.Context, tenantID, id int64) error
}

type StoreRoutingSettingsRepository interface {
	GetByTenant(ctx context.Context, tenantID int64) (*ListingKitStoreRoutingSettings, error)
	Upsert(ctx context.Context, settings *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error)
}

type inMemoryStoreProfileRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64][]ListingKitStoreProfile
}

func newInMemoryStoreProfileRepository() StoreProfileRepository {
	return &inMemoryStoreProfileRepository{
		nextID: 1,
		items:  make(map[int64][]ListingKitStoreProfile),
	}
}

func (r *inMemoryStoreProfileRepository) ListByTenant(_ context.Context, tenantID int64) ([]ListingKitStoreProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := append([]ListingKitStoreProfile(nil), r.items[tenantID]...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (r *inMemoryStoreProfileRepository) Upsert(_ context.Context, profile *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {
	if profile == nil {
		return nil, fmt.Errorf("store profile is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	copyProfile := *profile
	copyProfile.UpdatedAt = &now
	items := append([]ListingKitStoreProfile(nil), r.items[copyProfile.TenantID]...)
	for idx := range items {
		if copyProfile.ID > 0 && items[idx].ID == copyProfile.ID {
			copyProfile.Store = items[idx].Store
			items[idx] = copyProfile
			r.items[copyProfile.TenantID] = items
			return cloneStoreProfile(&copyProfile), nil
		}
		if copyProfile.ID == 0 && items[idx].StoreID == copyProfile.StoreID {
			copyProfile.ID = items[idx].ID
			copyProfile.Store = items[idx].Store
			items[idx] = copyProfile
			r.items[copyProfile.TenantID] = items
			return cloneStoreProfile(&copyProfile), nil
		}
	}
	copyProfile.ID = r.nextID
	r.nextID++
	items = append(items, copyProfile)
	r.items[copyProfile.TenantID] = items
	return cloneStoreProfile(&copyProfile), nil
}

func (r *inMemoryStoreProfileRepository) Delete(_ context.Context, tenantID, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.items[tenantID]
	for idx := range items {
		if items[idx].ID != id {
			continue
		}
		r.items[tenantID] = append(items[:idx], items[idx+1:]...)
		return nil
	}
	return fmt.Errorf("store profile not found")
}

type inMemoryStoreRoutingSettingsRepository struct {
	mu    sync.RWMutex
	items map[int64]ListingKitStoreRoutingSettings
}

func newInMemoryStoreRoutingSettingsRepository() StoreRoutingSettingsRepository {
	return &inMemoryStoreRoutingSettingsRepository{
		items: make(map[int64]ListingKitStoreRoutingSettings),
	}
}

func (r *inMemoryStoreRoutingSettingsRepository) GetByTenant(_ context.Context, tenantID int64) (*ListingKitStoreRoutingSettings, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	settings, ok := r.items[tenantID]
	if !ok {
		defaultSettings := defaultStoreRoutingSettings(tenantID)
		return &defaultSettings, nil
	}
	return cloneRoutingSettings(&settings), nil
}

func (r *inMemoryStoreRoutingSettingsRepository) Upsert(_ context.Context, settings *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	if settings == nil {
		return nil, fmt.Errorf("store routing settings are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copySettings := normalizeStoreRoutingSettings(*settings)
	now := time.Now()
	copySettings.UpdatedAt = &now
	r.items[copySettings.TenantID] = copySettings
	return cloneRoutingSettings(&copySettings), nil
}

func cloneStoreProfile(profile *ListingKitStoreProfile) *ListingKitStoreProfile {
	if profile == nil {
		return nil
	}
	copyProfile := *profile
	copyProfile.MatchRules = append([]ListingKitStoreMatchRule(nil), profile.MatchRules...)
	if profile.Store != nil {
		store := *profile.Store
		copyProfile.Store = &store
	}
	return &copyProfile
}

func cloneRoutingSettings(settings *ListingKitStoreRoutingSettings) *ListingKitStoreRoutingSettings {
	if settings == nil {
		return nil
	}
	copySettings := *settings
	return &copySettings
}

func normalizeStoreProfile(profile *ListingKitStoreProfile) {
	if profile == nil {
		return
	}
	profile.Site = strings.ToUpper(strings.TrimSpace(profile.Site))
	profile.WarehouseCode = strings.TrimSpace(profile.WarehouseCode)
	profile.DefaultSubmitMode = strings.ToLower(strings.TrimSpace(profile.DefaultSubmitMode))
	if profile.DefaultSubmitMode == "" {
		profile.DefaultSubmitMode = "publish"
	}
	if profile.DefaultStock < 0 {
		profile.DefaultStock = 0
	}
}

func defaultStoreRoutingSettings(tenantID int64) ListingKitStoreRoutingSettings {
	now := time.Now()
	return ListingKitStoreRoutingSettings{
		TenantID:            tenantID,
		SelectionStrategy:   "manual",
		AllowManualOverride: true,
		AllowFallback:       true,
		UpdatedAt:           &now,
	}
}

func normalizeStoreRoutingSettings(settings ListingKitStoreRoutingSettings) ListingKitStoreRoutingSettings {
	settings.SelectionStrategy = strings.ToLower(strings.TrimSpace(settings.SelectionStrategy))
	if settings.SelectionStrategy == "" {
		settings.SelectionStrategy = "manual"
	}
	return settings
}
