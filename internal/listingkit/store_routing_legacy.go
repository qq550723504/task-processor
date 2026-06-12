package listingkit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ListingKitStoreRoutingSettings struct {
	TenantID            int64      `json:"tenant_id,omitempty"`
	SelectionStrategy   string     `json:"selection_strategy,omitempty"`
	FallbackStoreID     int64      `json:"fallback_store_id,omitempty"`
	AllowManualOverride bool       `json:"allow_manual_override,omitempty"`
	AllowFallback       bool       `json:"allow_fallback,omitempty"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

type StoreRoutingSettingsRepository interface {
	GetByTenant(ctx context.Context, tenantID int64) (*ListingKitStoreRoutingSettings, error)
	Upsert(ctx context.Context, settings *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error)
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
		return nil, fmt.Errorf("legacy store routing settings are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copySettings := normalizeStoreRoutingSettings(*settings)
	now := time.Now()
	copySettings.UpdatedAt = &now
	r.items[copySettings.TenantID] = copySettings
	return cloneRoutingSettings(&copySettings), nil
}

func cloneRoutingSettings(settings *ListingKitStoreRoutingSettings) *ListingKitStoreRoutingSettings {
	if settings == nil {
		return nil
	}
	copySettings := *settings
	return &copySettings
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
