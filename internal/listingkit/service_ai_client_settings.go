package listingkit

import (
	"context"
)

func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {
	return s.settingsAdminOrDefault().GetAIClientSettings(ctx, scope, clientName)
}

func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {
	return s.settingsAdminOrDefault().UpdateAIClientSettings(ctx, req)
}
