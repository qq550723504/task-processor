package listingkit

import "context"

func (s *service) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {
	return s.settingsAdminOrDefault().GetSheinStoreRoutingSettings(ctx)
}

func (s *service) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	return s.settingsAdminOrDefault().UpdateSheinStoreRoutingSettings(ctx, req)
}
