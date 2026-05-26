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
		repo:                              s.repo,
		assetRepo:                         s.assetRepo,
		assetRecipeResolver:               s.assetRecipeResolver,
		assetBundleBuilder:                s.assetBundleBuilder,
		assetGenerator:                    s.assetGenerator,
		listAssetGenerationTasks:          s.listAssetGenerationTasks,
		listGenerationReviews:             s.listGenerationReviews,
		buildRetryGenerationTaskSelection: s.buildRetryGenerationTaskSelection,
		persistGenerationReviewDecision:   s.persistGenerationReviewDecision,
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return s.standardProductWorkflowClient, s.standardProductWorkflowEnabled
		},
		platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
			return s.platformAdaptWorkflowClient, s.platformAdaptWorkflowEnabled
		},
	})
	return s.taskGeneration
}
