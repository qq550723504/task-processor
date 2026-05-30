package listingkit

type taskGenerationCurrentStateViewsPhase struct{}

func buildTaskGenerationCurrentStateViewsPhase() *taskGenerationCurrentStateViewsPhase {
	return &taskGenerationCurrentStateViewsPhase{}
}

func (p *taskGenerationCurrentStateViewsPhase) overview(result *ListingKitResult) *AssetGenerationOverview {
	if result == nil {
		return nil
	}
	return result.AssetGenerationOverview
}

func (p *taskGenerationCurrentStateViewsPhase) queue(result *ListingKitResult) *GenerationWorkQueue {
	if result == nil {
		return nil
	}
	return result.AssetGenerationQueue
}

func (p *taskGenerationCurrentStateViewsPhase) renderPreviews(result *ListingKitResult, query *GenerationQueueQuery) []PlatformAssetRenderPreviews {
	return buildActionPlatformRenderPreviews(result, query)
}
