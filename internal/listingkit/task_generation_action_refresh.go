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

	hydration := buildTaskGenerationActionRefreshHydrationPhase()
	hydration.query = query
	return hydration.run(baseResult, refresh), nil
}
