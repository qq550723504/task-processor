package listingkit

import (
	"context"
	"fmt"
)

func (s *service) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	items, err := s.storeProfileRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return s.attachStoreOptions(ctx, items), nil
}

func (s *service) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("store profile is required")
	}
	if req.StoreID <= 0 {
		return nil, fmt.Errorf("store_id is required")
	}
	profile := *req
	profile.TenantID = tenantID
	normalizeStoreProfile(&profile)
	saved, err := s.storeProfileRepo.Upsert(ctx, &profile)
	if err != nil {
		return nil, err
	}
	items := s.attachStoreOptions(ctx, []ListingKitStoreProfile{*saved})
	if len(items) == 0 {
		return saved, nil
	}
	return cloneStoreProfile(&items[0]), nil
}

func (s *service) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return fmt.Errorf("tenant id is required")
	}
	if id <= 0 {
		return fmt.Errorf("profile id is required")
	}
	return s.storeProfileRepo.Delete(ctx, tenantID, id)
}

func (s *service) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	return s.routingSettingsRepo.GetByTenant(ctx, tenantID)
}

func (s *service) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("store routing settings are required")
	}
	settings := *req
	settings.TenantID = tenantID
	settings = normalizeStoreRoutingSettings(settings)
	return s.routingSettingsRepo.Upsert(ctx, &settings)
}

func (s *service) attachStoreOptions(ctx context.Context, items []ListingKitStoreProfile) []ListingKitStoreProfile {
	if len(items) == 0 {
		return items
	}
	options := s.listSheinStoreOptions(ctx)
	if len(options) == 0 {
		return items
	}
	byID := make(map[int64]SheinStoreOption, len(options))
	for _, option := range options {
		byID[option.ID] = option
	}
	for idx := range items {
		option, ok := byID[items[idx].StoreID]
		if !ok {
			continue
		}
		copyOption := option
		items[idx].Store = &copyOption
	}
	return items
}
