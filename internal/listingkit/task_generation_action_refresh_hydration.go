package listingkit

type taskGenerationActionRefreshHydrationPhase struct {
	query *GenerationQueueQuery
}

func buildTaskGenerationActionRefreshHydrationPhase() *taskGenerationActionRefreshHydrationPhase {
	return &taskGenerationActionRefreshHydrationPhase{}
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
