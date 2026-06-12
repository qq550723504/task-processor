package listingkit

import (
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
