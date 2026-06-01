package listingkit

import "context"

type taskGenerationQueueReadSnapshot struct {
	task   *Task
	result *ListingKitResult
	queue  *GenerationWorkQueue
}

type taskGenerationQueueReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationQueueReadSnapshotPhase(service *taskGenerationService) *taskGenerationQueueReadSnapshotPhase {
	return &taskGenerationQueueReadSnapshotPhase{service: service}
}

func (p *taskGenerationQueueReadSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationQueueReadSnapshot, error) {
	if p == nil || p.service == nil {
		return &taskGenerationQueueReadSnapshot{task: &Task{ID: taskID}}, nil
	}
	task, err := p.service.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := p.service.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	reviews, err := p.service.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	reviewedResult := withListingKitResultGenerationAndReview(task.Result, tasks, reviews)
	return &taskGenerationQueueReadSnapshot{
		task:   task,
		result: reviewedResult,
		queue:  reviewedResult.AssetGenerationQueue,
	}, nil
}
