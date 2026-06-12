package listingkit

import (
	"context"
	"fmt"
)

func (s *settingsAdminService) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {
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

func (s *settingsAdminService) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {
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

func (s *settingsAdminService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return fmt.Errorf("tenant id is required")
	}
	if id <= 0 {
		return fmt.Errorf("profile id is required")
	}
	return s.storeProfileRepo.Delete(ctx, tenantID, id)
}
