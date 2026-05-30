package listingkit

import "context"

type taskGenerationActionRefreshExtractPhase struct {
	service *taskGenerationService
}

type taskGenerationActionRefreshExtractResult struct {
	currentResult          *ListingKitResult
	overview               *AssetGenerationOverview
	platformRenderPreviews []PlatformAssetRenderPreviews
}

func buildTaskGenerationActionRefreshExtractPhase(service *taskGenerationService) *taskGenerationActionRefreshExtractPhase {
	return &taskGenerationActionRefreshExtractPhase{service: service}
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
