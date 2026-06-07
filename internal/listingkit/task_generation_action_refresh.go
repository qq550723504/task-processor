package listingkit

import "context"

type taskGenerationActionRefreshPhase struct {
	service *taskGenerationService
}

type taskGenerationActionRefreshExtractPhase struct {
	service *taskGenerationService
}

type taskGenerationActionRefreshHydrationPhase struct {
	query *GenerationQueueQuery
}

type taskGenerationActionRefreshExtractResult struct {
	currentResult          *ListingKitResult
	overview               *AssetGenerationOverview
	platformRenderPreviews []PlatformAssetRenderPreviews
}

type taskGenerationActionRefreshResult struct {
	overview               *AssetGenerationOverview
	platformRenderPreviews []PlatformAssetRenderPreviews
	currentResult          *ListingKitResult
}

func buildTaskGenerationActionRefreshPhase(service *taskGenerationService) *taskGenerationActionRefreshPhase {
	return &taskGenerationActionRefreshPhase{service: service}
}

func buildTaskGenerationActionRefreshExtractPhase(service *taskGenerationService) *taskGenerationActionRefreshExtractPhase {
	return &taskGenerationActionRefreshExtractPhase{service: service}
}

func buildTaskGenerationActionRefreshHydrationPhase() *taskGenerationActionRefreshHydrationPhase {
	return &taskGenerationActionRefreshHydrationPhase{}
}

func (p *taskGenerationActionRefreshPhase) run(ctx context.Context, taskID string, baseResult *ListingKitResult, query *GenerationQueueQuery) (*taskGenerationActionRefreshResult, error) {
	refresh, err := buildTaskGenerationActionRefreshExtractPhase(p.service).run(ctx, taskID, query)
	if err != nil {
		return nil, err
	}

	hydration := buildTaskGenerationActionRefreshHydrationPhase()
	hydration.query = query
	return hydration.run(baseResult, refresh), nil
}

func (p *taskGenerationActionRefreshExtractPhase) run(ctx context.Context, taskID string, query *GenerationQueueQuery) (*taskGenerationActionRefreshExtractResult, error) {
	currentResult, err := p.service.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}

	overview := currentResult.AssetGenerationOverview
	platformRenderPreviews := buildActionPlatformRenderPreviews(currentResult, query)

	return &taskGenerationActionRefreshExtractResult{
		currentResult:          currentResult,
		overview:               overview,
		platformRenderPreviews: platformRenderPreviews,
	}, nil
}

func (p *taskGenerationActionRefreshHydrationPhase) run(baseResult *ListingKitResult, refresh *taskGenerationActionRefreshExtractResult) *taskGenerationActionRefreshResult {
	currentResult := refresh.currentResult
	platformRenderPreviews := refresh.platformRenderPreviews
	if len(platformRenderPreviews) == 0 {
		platformRenderPreviews = buildActionPlatformRenderPreviews(baseResult, p.query)
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
	}
}
