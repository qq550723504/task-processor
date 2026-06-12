package listingkit

import "context"

func (s *service) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {
	return s.settingsAdminOrDefault().ListSheinStoreProfiles(ctx)
}

func (s *service) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {
	return s.settingsAdminOrDefault().UpsertSheinStoreProfile(ctx, req)
}

func (s *service) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return s.settingsAdminOrDefault().DeleteSheinStoreProfile(ctx, id)
}
