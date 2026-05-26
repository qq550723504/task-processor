package listingkit

import "context"

func (s *service) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationTasks(ctx, taskID, query)
}

func (s *service) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {
	return s.taskGenerationOrDefault().GetTaskGenerationQueue(ctx, taskID, query)
}

func (s *service) taskGenerationOrDefault() *taskGenerationService {
	if s.taskGeneration != nil {
		return s.taskGeneration
	}
	s.taskGeneration = newTaskGenerationService(taskGenerationServiceConfig{
		repo:                     s.repo,
		listAssetGenerationTasks: s.listAssetGenerationTasks,
		listGenerationReviews:    s.listGenerationReviews,
		getCurrentListingKitResult: func(ctx context.Context, taskID string) (*ListingKitResult, error) {
			return s.getCurrentListingKitResult(ctx, taskID)
		},
		getCurrentAssetGenerationQueue: func(ctx context.Context, taskID string) (*GenerationWorkQueue, error) {
			return s.getCurrentAssetGenerationQueue(ctx, taskID)
		},
	})
	return s.taskGeneration
}
