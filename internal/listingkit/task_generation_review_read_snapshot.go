package listingkit

import "context"

type taskGenerationReviewReadSnapshot struct {
	taskID string
	result *ListingKitResult
	queue  *GenerationWorkQueue
}

type taskGenerationReviewReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationReviewReadSnapshotPhase(service *taskGenerationService) *taskGenerationReviewReadSnapshotPhase {
	return &taskGenerationReviewReadSnapshotPhase{service: service}
}

func (p *taskGenerationReviewReadSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationReviewReadSnapshot, error) {
	if p == nil || p.service == nil {
		return &taskGenerationReviewReadSnapshot{taskID: taskID}, nil
	}
	result, err := p.service.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return &taskGenerationReviewReadSnapshot{
		taskID: taskID,
		result: result,
		queue:  result.AssetGenerationQueue,
	}, nil
}
