package listingkit

import "context"

func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	return s.settingsAdminOrDefault().GetSheinSettings(ctx)
}

func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {
	return s.settingsAdminOrDefault().UpdateSheinSettings(ctx, req)
}
