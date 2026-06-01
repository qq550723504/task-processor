package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskGenerationCurrentStateSnapshot struct {
	task    *Task
	result  *ListingKitResult
	tasks   []assetgeneration.Task
	reviews []GenerationReviewRecord
}

type taskGenerationCurrentStateSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationCurrentStateSnapshotPhase(service *taskGenerationService) *taskGenerationCurrentStateSnapshotPhase {
	return &taskGenerationCurrentStateSnapshotPhase{service: service}
}

func (p *taskGenerationCurrentStateSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationCurrentStateSnapshot, error) {
	if p == nil || p.service == nil {
		return &taskGenerationCurrentStateSnapshot{task: &Task{ID: taskID}}, nil
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
	result := withListingKitResultGenerationAndReview(task.Result, tasks, reviews)

	return &taskGenerationCurrentStateSnapshot{
		task:    task,
		result:  result,
		tasks:   tasks,
		reviews: reviews,
	}, nil
}
