package listingkit

import (
	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildTaskDependencies(config *ServiceConfig) taskDependencies {
	if config == nil {
		return taskDependencies{}
	}
	return taskDependencies{
		productSnapshots:             config.Core.ProductSnapshotReader,
		sdsLoginStatusProvider:       config.Core.SDSLoginStatusProvider,
		taskSubmitter:                config.Core.TaskSubmitter,
		generationUsage:              config.Core.GenerationUsageLedger,
		generationUsageAdmission:     config.Core.GenerationUsageAdmission,
		standardWorkflowClient:       config.Workflow.StandardProductWorkflowClient,
		standardWorkflowEnabled:      config.Workflow.StandardProductWorkflowEnabled,
		platformAdaptWorkflowClient:  config.Workflow.PlatformAdaptWorkflowClient,
		platformAdaptWorkflowEnabled: config.Workflow.PlatformAdaptWorkflowEnabled,
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
		sheinSubmitLocks: listingsubmission.NewSubmitLockManager(),
	}
}

func buildSubmissionDependencies(config *ServiceConfig) submissionDependencies {
	if config == nil {
		return submissionDependencies{}
	}
	return submissionDependencies{
		storeProfileRepo:            config.Core.StoreProfileRepository,
		sheinProductAPIBuilder:      config.Shein.SheinProductAPIBuilder,
		sheinImageAPIBuilder:        config.Shein.SheinImageAPIBuilder,
		sheinTranslateAPIBuilder:    config.Shein.SheinTranslateAPIBuilder,
		sheinPublishWorkflowClient:  config.Workflow.SheinPublishWorkflowClient,
		sheinPublishWorkflowEnabled: config.Workflow.SheinPublishWorkflowEnabled,
	}
}

func buildSheinSharedDependencies(config *ServiceConfig) sheinSharedDependencies {
	if config == nil {
		return sheinSharedDependencies{}
	}
	return sheinSharedDependencies{
		storeCatalog:         config.Shein.SheinStoreCatalog,
		storeAccessValidator: config.Shein.StoreAccessValidator,
		apiClientFactory:     config.Shein.SheinAPIClientFactory,
		contentOptimizer:     config.Shein.SheinContentOptimizer,
	}
}

func buildWorkflowDependencies(config *ServiceConfig) workflowDependencies {
	if config == nil {
		return workflowDependencies{}
	}
	return workflowDependencies{
		productSnapshots: config.Core.ProductSnapshotReader,
		approvedAssets:   config.Assets.ApprovedAssetInventoryReader,
	}
}

func buildSheinRuntimeDependencies(config *ServiceConfig) sheinRuntimeDependencies {
	if config == nil {
		return sheinRuntimeDependencies{}
	}
	dependencies := sheinRuntimeDependencies{
		resolutionCacheStore:  config.Shein.SheinResolutionCacheStore,
		categoryResolver:      config.Shein.SheinCategoryResolver,
		attributeResolver:     config.Shein.SheinAttributeResolver,
		saleAttributeResolver: config.Shein.SheinSaleAttributeResolver,
		sizeHeaderResolver:    config.Shein.SheinSizeHeaderResolver,
		pricingPolicy:         config.Shein.SheinPricingPolicy,
	}
	dependencies.freshAttributeResolver, _ = config.Shein.SheinAttributeResolver.(sheinpub.FreshAttributeResolver)
	return dependencies
}

func buildSupportDependencies(config *ServiceConfig) supportDependencies {
	if config == nil {
		return supportDependencies{}
	}
	return supportDependencies{
		sdsSyncService:            config.Core.SDSSyncService,
		sdsBaselineRemoteProvider: config.Core.SDSBaselineRemoteProvider,
		imageUploadStore:          config.Core.ImageUploadStore,
		uploadedImageRepository:   config.Core.UploadedImageRepository,
		assembler:                 config.Assets.Assembler,
		reviewRepository:          config.Assets.ReviewRepository,
	}
}

func applyServiceDependencyGroups(svc *service, config *ServiceConfig) {
	if svc == nil {
		return
	}
	svc.taskDeps = buildTaskDependencies(config)
	svc.submission = buildSubmissionCollaborators()
	svc.adminDeps = buildAdminDependencies(config)
	svc.submissionDeps = buildSubmissionDependencies(config)
	svc.sheinSharedDeps = buildSheinSharedDependencies(config)
	svc.workflowDeps = buildWorkflowDependencies(config)
	svc.sheinRuntimeDeps = buildSheinRuntimeDependencies(config)
	svc.supportDeps = buildSupportDependencies(config)
}
