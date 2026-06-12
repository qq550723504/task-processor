package listingkit

import (
	"context"
	"fmt"
	"time"
)

func (s *settingsAdminService) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	settings := defaultStoreRoutingSettings(tenantID)
	return &settings, nil
}

func (s *settingsAdminService) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("legacy store routing settings are required")
	}
	settings := defaultStoreRoutingSettings(tenantID)
	now := time.Now()
	settings.UpdatedAt = &now
	return &settings, nil
}
