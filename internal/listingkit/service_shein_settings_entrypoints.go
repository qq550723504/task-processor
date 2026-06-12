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

func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {
	return s.settingsAdminOrDefault().GetAIClientSettings(ctx, scope, clientName)
}

func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {
	return s.settingsAdminOrDefault().UpdateAIClientSettings(ctx, req)
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

func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {
	return s.sheinAdminOrDefault().GetSubmissionEvents(ctx, taskID)
}

func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {
	return s.sheinAdminOrDefault().UpdateSheinFinalDraft(ctx, taskID, req)
}
