package listingkit

import "task-processor/internal/listingkit/submission"

func buildGenerateRequestDefaults(config *ServiceConfig) generateRequestDefaults {
	if config == nil {
		return generateRequestDefaults{}
	}
	return generateRequestDefaults{
		sheinDefaultStoreID: config.Shein.SheinDefaultStoreID,
	}
}

func buildTaskDependencies(config *ServiceConfig) taskDependencies {
	if config == nil {
		return taskDependencies{}
	}
	return taskDependencies{
		sdsLoginStatusProvider:       config.Core.SDSLoginStatusProvider,
		taskSubmitter:                config.Core.TaskSubmitter,
		requestDefaults:              buildGenerateRequestDefaults(config),
		standardWorkflowClient:       config.Workflow.StandardProductWorkflowClient,
		standardWorkflowEnabled:      config.Workflow.StandardProductWorkflowEnabled,
		platformAdaptWorkflowClient:  config.Workflow.PlatformAdaptWorkflowClient,
		platformAdaptWorkflowEnabled: config.Workflow.PlatformAdaptWorkflowEnabled,
	}
}

func buildStudioDependencies(config *ServiceConfig) studioDependencies {
	if config == nil {
		return studioDependencies{}
	}
	return studioDependencies{
		sessionRepo:       config.Core.StudioSessionRepository,
		batchRepo:         config.Core.StudioBatchRepository,
		batchRunRepo:      config.Core.StudioBatchRunRepository,
		promptDiversifier: config.Shein.StudioPromptDiversifier,
		imageGenerator:    config.Shein.StudioImageGenerator,
		uploadStore:       config.Core.ImageUploadStore,
	}
}

func buildAdminDependencies(config *ServiceConfig) adminDependencies {
	if config == nil {
		return adminDependencies{}
	}
	return adminDependencies{
		storeProfileRepo:  config.Core.StoreProfileRepository,
		aiCredentialStore: config.Core.AIClientCredentialStore,
	}
}

func buildSubmissionCollaborators() submissionCollaborators {
	return submissionCollaborators{
		sheinSubmitLocks: submission.NewSubmitLockManager(),
	}
}

func buildSubmissionDependencies(config *ServiceConfig) submissionDependencies {
	if config == nil {
		return submissionDependencies{}
	}
	return submissionDependencies{
		storeProfileRepo:            config.Core.StoreProfileRepository,
		sheinStoreCatalog:           config.Shein.SheinStoreCatalog,
		sheinAPIClientFactory:       config.Shein.SheinAPIClientFactory,
		sheinProductAPIBuilder:      config.Shein.SheinProductAPIBuilder,
		sheinImageAPIBuilder:        config.Shein.SheinImageAPIBuilder,
		sheinTranslateAPIBuilder:    config.Shein.SheinTranslateAPIBuilder,
		sheinContentOptimizer:       config.Shein.SheinContentOptimizer,
		sheinPublishWorkflowClient:  config.Workflow.SheinPublishWorkflowClient,
		sheinPublishWorkflowEnabled: config.Workflow.SheinPublishWorkflowEnabled,
	}
}

func buildWorkflowDependencies(config *ServiceConfig) workflowDependencies {
	if config == nil {
		return workflowDependencies{}
	}
	return workflowDependencies{
		productService:         config.Core.ProductService,
		imageService:           config.Core.ImageService,
		assetRepository:        config.Assets.AssetRepository,
		assetRecipeResolver:    config.Assets.AssetRecipeResolver,
		assetBundleBuilder:     config.Assets.AssetBundleBuilder,
		assetGenerationService: config.Assets.AssetGenerationService,
		sheinContentOptimizer:  config.Shein.SheinContentOptimizer,
	}
}

func buildSheinRuntimeDependencies(config *ServiceConfig) sheinRuntimeDependencies {
	if config == nil {
		return sheinRuntimeDependencies{}
	}
	return sheinRuntimeDependencies{
		resolutionCacheStore:  config.Shein.SheinResolutionCacheStore,
		storeCatalog:          config.Shein.SheinStoreCatalog,
		apiClientFactory:      config.Shein.SheinAPIClientFactory,
		categoryResolver:      config.Shein.SheinCategoryResolver,
		attributeResolver:     config.Shein.SheinAttributeResolver,
		saleAttributeResolver: config.Shein.SheinSaleAttributeResolver,
		pricingPolicy:         config.Shein.SheinPricingPolicy,
	}
}

func buildSupportDependencies(config *ServiceConfig) supportDependencies {
	if config == nil {
		return supportDependencies{}
	}
	return supportDependencies{
		sdsSyncService:            config.Core.SDSSyncService,
		sdsBaselineRemoteProvider: config.Core.SDSBaselineRemoteProvider,
		uploadedImageRepository:   config.Core.UploadedImageRepository,
		assembler:                 config.Assets.Assembler,
		reviewRepository:          config.Assets.ReviewRepository,
	}
}

func applyServiceDependencyGroups(svc *service, config *ServiceConfig) {
	if svc == nil {
		return
	}
	svc.requestDefaults = buildGenerateRequestDefaults(config)
	svc.taskDeps = buildTaskDependencies(config)
	svc.studioDeps = buildStudioDependencies(config)
	svc.submission = buildSubmissionCollaborators()
	svc.adminDeps = buildAdminDependencies(config)
	svc.submissionDeps = buildSubmissionDependencies(config)
	svc.workflowDeps = buildWorkflowDependencies(config)
	svc.sheinRuntimeDeps = buildSheinRuntimeDependencies(config)
	svc.supportDeps = buildSupportDependencies(config)
}
