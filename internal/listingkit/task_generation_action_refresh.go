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
	refresh, err := buildTaskGenerationActionRefreshExtractPhase(p.service).run(ctx, taskID, query)
	if err != nil {
		return nil, err
	}

	currentResult := refresh.currentResult
	platformRenderPreviews := refresh.platformRenderPreviews
	if len(platformRenderPreviews) == 0 {
		platformRenderPreviews = buildActionPlatformRenderPreviews(baseResult, query)
	}
	if len(currentResult.PlatformAssetRenderPreviews) == 0 && len(platformRenderPreviews) > 0 {
		currentResult.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), platformRenderPreviews...)
	}
	if len(currentResult.AssetRenderPreviews) == 0 && baseResult != nil {
		currentResult.AssetRenderPreviews = append([]AssetRenderPreview(nil), baseResult.AssetRenderPreviews...)
	}
	return &taskGenerationActionRefreshResult{
		overview:               refresh.overview,
		platformRenderPreviews: platformRenderPreviews,
		currentResult:          currentResult,
	}, nil
}
