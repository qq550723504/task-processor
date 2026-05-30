package listingkit

import "context"

type taskGenerationActionRefreshPhase struct {
	service *taskGenerationService
}

type taskGenerationActionRefreshResult struct {
	overview               *AssetGenerationOverview
	platformRenderPreviews []PlatformAssetRenderPreviews
	currentResult          *ListingKitResult
}

func buildTaskGenerationActionRefreshPhase(service *taskGenerationService) *taskGenerationActionRefreshPhase {
	return &taskGenerationActionRefreshPhase{service: service}
}

func (p *taskGenerationActionRefreshPhase) run(ctx context.Context, taskID string, baseResult *ListingKitResult, query *GenerationQueueQuery) (*taskGenerationActionRefreshResult, error) {
	overview, err := p.service.getCurrentAssetGenerationOverview(ctx, taskID)
	if err != nil {
		return nil, err
	}
	platformRenderPreviews, err := p.service.getCurrentActionRenderPreviews(ctx, taskID, query)
	if err != nil {
		return nil, err
	}
	if len(platformRenderPreviews) == 0 {
		platformRenderPreviews = buildActionPlatformRenderPreviews(baseResult, query)
	}
	currentResult, err := p.service.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(currentResult.PlatformAssetRenderPreviews) == 0 && len(platformRenderPreviews) > 0 {
		currentResult.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), platformRenderPreviews...)
	}
	if len(currentResult.AssetRenderPreviews) == 0 && baseResult != nil {
		currentResult.AssetRenderPreviews = append([]AssetRenderPreview(nil), baseResult.AssetRenderPreviews...)
	}
	return &taskGenerationActionRefreshResult{
		overview:               overview,
		platformRenderPreviews: platformRenderPreviews,
		currentResult:          currentResult,
	}, nil
}
