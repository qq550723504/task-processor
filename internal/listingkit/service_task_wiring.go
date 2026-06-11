package listingkit

import "context"

func buildTaskGenerationServiceConfig(s *service) taskGenerationServiceConfig {
	return taskGenerationServiceConfig{
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
	}
}

func buildTaskRevisionServiceConfig(s *service) taskRevisionServiceConfig {
	return taskRevisionServiceConfig{
		repo: s.repo,
		resolveManualSheinSaleAttributeValueIDs: func(ctx context.Context, task *Task, req *ApplyRevisionRequest) error {
			return s.resolveManualSheinSaleAttributeValueIDs(ctx, task, req)
		},
		mutateTaskResult: s.mutateTaskResult,
		refreshSheinDerivedState: func(task *Task, req *ApplyRevisionRequest) {
			s.refreshSheinDerivedState(task, req)
		},
		buildTaskPreview: s.buildTaskPreview,
	}
}

func buildTaskLifecycleServiceConfig(s *service) taskLifecycleServiceConfig {
	return taskLifecycleServiceConfig{
		repo:                   s.repo,
		sdsLoginStatusProvider: s.sdsLoginStatusProvider,
		requestDefaults: func() generateRequestDefaults {
			return s.requestDefaults
		},
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return s.standardProductWorkflowClient, s.standardProductWorkflowEnabled
		},
		processListingKit: s.ProcessListingKit,
		resolveStoreSelection: func(ctx context.Context, task *Task) (*sheinStoreSelection, error) {
			return s.resolveSheinStoreSelection(ctx, task)
		},
		buildResultPayload: func(ctx context.Context, task *Task) (*ListingKitResult, error) {
			return s.buildTaskResultPayload(ctx, task)
		},
	}
}

func buildSDSBaselineServiceConfig(s *service) sdsBaselineServiceConfig {
	if s == nil {
		return sdsBaselineServiceConfig{}
	}
	return sdsBaselineServiceConfig{
		repo:                   s.repo,
		sdsLoginStatusProvider: s.sdsLoginStatusProvider,
	}
}
