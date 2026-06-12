package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	return s.settingsAdminOrDefault().GetSheinSettings(ctx)
}

func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {
	return s.settingsAdminOrDefault().UpdateSheinSettings(ctx, req)
}

func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {
	return s.sheinAdminOrDefault().SearchSheinCategories(ctx, taskID, query)
}

func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return s.sheinAdminOrDefault().PreviewSheinPrice(ctx, taskID, req)
}

func (s *service) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {
	return s.sheinAdminOrDefault().ClearSheinResolutionCache(ctx, taskID, kind)
}
