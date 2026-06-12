package listingkit

import (
	"context"
)

func buildTaskGenerationServiceConfig(s *service) taskGenerationServiceConfig {
	return taskGenerationServiceConfig{
		repo:                              s.repo,
		assetRepo:                         resolveWorkflowAssetRepository(s),
		assetRecipeResolver:               resolveWorkflowAssetRecipeResolver(s),
		assetBundleBuilder:                resolveWorkflowAssetBundleBuilder(s),
		assetGenerator:                    resolveWorkflowAssetGenerationService(s),
		listAssetGenerationTasks:          s.listAssetGenerationTasks,
		listGenerationReviews:             s.listGenerationReviews,
		buildRetryGenerationTaskSelection: s.buildRetryGenerationTaskSelection,
		persistGenerationReviewDecision:   s.persistGenerationReviewDecision,
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return resolveStandardWorkflowClient(s)
		},
		platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
			return resolvePlatformAdaptWorkflowClient(s)
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
		buildTaskPreview: func(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
			return s.buildTaskPreview(ctx, task, platform)
		},
	}
}

func buildTaskPreviewServiceConfig(s *service) taskPreviewServiceConfig {
	wiring := buildTaskPreviewExportReadWiring(s)
	return taskPreviewServiceConfig{
		repo:                     wiring.repo,
		listAssetGenerationTasks: wiring.listAssetGenerationTasks,
		decorateSheinCookieAvailabilityPreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) {
			s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
		},
		decorateSheinStoreResolutionPreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) {
			s.decorateSheinStoreResolutionPreview(ctx, task, preview)
		},
	}
}

func buildTaskExportServiceConfig(s *service) taskExportServiceConfig {
	wiring := buildTaskPreviewExportReadWiring(s)
	return taskExportServiceConfig{
		repo:                     wiring.repo,
		listAssetGenerationTasks: wiring.listAssetGenerationTasks,
	}
}

func buildTaskLifecycleServiceConfig(s *service) taskLifecycleServiceConfig {
	return taskLifecycleServiceConfig{
		repo:                        s.repo,
		sdsBaselineReadinessService: s.sdsBaselineOrDefault(),
		requestDefaults: func() generateRequestDefaults {
			return resolveTaskRequestDefaults(s)
		},
		taskSubmitter: func() TaskSubmitter {
			return resolveTaskSubmitter(s)
		},
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return resolveStandardWorkflowClient(s)
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
		sdsLoginStatusProvider: resolveSDSLoginStatusProvider(s),
	}
}
