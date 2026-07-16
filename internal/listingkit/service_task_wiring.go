package listingkit

import (
	"context"
)

func buildTaskGenerationServiceConfig(s *service) taskGenerationServiceConfig {
	repository := buildTaskRepositoryWiring(s)
	return taskGenerationServiceConfig{
		repo:                              repository.repo,
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
	preview := buildTaskPreviewAccessWiring(s)
	repository := buildTaskRepositoryWiring(s)
	return taskRevisionServiceConfig{
		repo: repository.repo,
		resolveManualSheinSaleAttributeValueIDs: func(ctx context.Context, task *Task, req *ApplyRevisionRequest) error {
			return s.resolveManualSheinSaleAttributeValueIDs(ctx, task, req)
		},
		recovery: s.taskSubmissionRecoveryOrDefault(),
		refreshSheinDerivedState: func(task *Task, req *ApplyRevisionRequest) {
			s.refreshSheinDerivedState(task, req)
		},
		refreshSheinTaskResultState: func(ctx context.Context, task *Task, result *ListingKitResult) {
			s.refreshSheinTaskResultState(ctx, task, result)
		},
		buildTaskPreview: preview.buildTaskPreview,
	}
}

func buildTaskPreviewServiceConfig(s *service) taskPreviewServiceConfig {
	repository := buildTaskRepositoryWiring(s)
	decorators := buildTaskPreviewDecorationWiring(s)
	return taskPreviewServiceConfig{
		repo:       repository.repo,
		decorators: decorators,
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
	repository := buildTaskRepositoryWiring(s)
	return taskLifecycleServiceConfig{
		repo:                        repository.repo,
		sdsBaselineReadinessService: s.sdsBaselineOrDefault(),
		validateSheinStoreAccess: func(ctx context.Context, tenantID, storeID int64) error {
			validator := resolveSheinStoreAccessValidator(s)
			if validator == nil {
				return NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
			}
			_, err := validator.ValidateStoreAccess(ctx, tenantID, storeID, "SHEIN")
			return err
		},
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
	repository := buildTaskRepositoryWiring(s)
	return sdsBaselineServiceConfig{
		repo:                   repository.repo,
		sdsLoginStatusProvider: resolveSDSLoginStatusProvider(s),
	}
}
