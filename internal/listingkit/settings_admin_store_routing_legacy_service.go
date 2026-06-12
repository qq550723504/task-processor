package listingkit

import (
	"context"
	"fmt"
)

func (s *settingsAdminService) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	return s.routingSettingsRepo.GetByTenant(ctx, tenantID)
}

func (s *settingsAdminService) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("legacy store routing settings are required")
	}
	settings := *req
	settings.TenantID = tenantID
	settings = normalizeStoreRoutingSettings(settings)
	return s.routingSettingsRepo.Upsert(ctx, &settings)
}
