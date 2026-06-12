package listingkit

import (
	"fmt"

	"task-processor/internal/listingkit/submission"
)

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Core.Repository == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if config.Core.ProductService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}
	config.applyDefaults()
	svc := newServiceWithConfig(config)
	svc.initializeCollaborators()
	return svc, nil
}

func newServiceWithConfig(config *ServiceConfig) *service {
	defaultSettings := defaultSheinSettings(config.Shein.SheinDefaultStoreID, config.Shein.SheinPricingPolicy)
	return &service{
		repo:                           config.Core.Repository,
		studioSessionRepo:              config.Core.StudioSessionRepository,
		studioBatchRepo:                config.Core.StudioBatchRepository,
		studioBatchRunRepo:             config.Core.StudioBatchRunRepository,
		productSvc:                     config.Core.ProductService,
		imageSvc:                       config.Core.ImageService,
		sdsSyncSvc:                     config.Core.SDSSyncService,
		sdsLoginStatusProvider:         config.Core.SDSLoginStatusProvider,
		sdsBaselineRemoteProvider:      config.Core.SDSBaselineRemoteProvider,
		uploadStore:                    config.Core.ImageUploadStore,
		uploadedImageRepo:              config.Core.UploadedImageRepository,
		assembler:                      config.Assets.Assembler,
		sheinCategoryResolver:          config.Shein.SheinCategoryResolver,
		sheinResolutionCacheStore:      config.Shein.SheinResolutionCacheStore,
		sheinStoreCatalog:              config.Shein.SheinStoreCatalog,
		sheinAPIClientFactory:          config.Shein.SheinAPIClientFactory,
		sheinAttributeResolver:         config.Shein.SheinAttributeResolver,
		sheinSaleAttributeResolver:     config.Shein.SheinSaleAttributeResolver,
		sheinPricingPolicy:             config.Shein.SheinPricingPolicy,
		sheinProductAPIBuilder:         config.Shein.SheinProductAPIBuilder,
		sheinImageAPIBuilder:           config.Shein.SheinImageAPIBuilder,
		sheinTranslateAPIBuilder:       config.Shein.SheinTranslateAPIBuilder,
		sheinContentOptimizer:          config.Shein.SheinContentOptimizer,
		studioPromptDiversifier:        config.Shein.StudioPromptDiversifier,
		studioImageGenerator:           config.Shein.StudioImageGenerator,
		aiCredentialStore:              config.Core.AIClientCredentialStore,
		assetRepo:                      config.Assets.AssetRepository,
		reviewRepo:                     config.Assets.ReviewRepository,
		assetRecipeResolver:            config.Assets.AssetRecipeResolver,
		assetBundleBuilder:             config.Assets.AssetBundleBuilder,
		assetGenerator:                 config.Assets.AssetGenerationService,
		taskSubmitter:                  config.Core.TaskSubmitter,
		sheinPublishWorkflowClient:     config.Workflow.SheinPublishWorkflowClient,
		sheinPublishWorkflowEnabled:    config.Workflow.SheinPublishWorkflowEnabled,
		standardProductWorkflowClient:  config.Workflow.StandardProductWorkflowClient,
		standardProductWorkflowEnabled: config.Workflow.StandardProductWorkflowEnabled,
		platformAdaptWorkflowClient:    config.Workflow.PlatformAdaptWorkflowClient,
		platformAdaptWorkflowEnabled:   config.Workflow.PlatformAdaptWorkflowEnabled,
		storeProfileRepo:               config.Core.StoreProfileRepository,
		requestDefaults: generateRequestDefaults{
			sheinDefaultStoreID: config.Shein.SheinDefaultStoreID,
		},
		taskDeps: taskDependencies{
			sdsLoginStatusProvider: config.Core.SDSLoginStatusProvider,
			taskSubmitter:          config.Core.TaskSubmitter,
			requestDefaults: generateRequestDefaults{
				sheinDefaultStoreID: config.Shein.SheinDefaultStoreID,
			},
			standardWorkflowClient:       config.Workflow.StandardProductWorkflowClient,
			standardWorkflowEnabled:      config.Workflow.StandardProductWorkflowEnabled,
			platformAdaptWorkflowClient:  config.Workflow.PlatformAdaptWorkflowClient,
			platformAdaptWorkflowEnabled: config.Workflow.PlatformAdaptWorkflowEnabled,
		},
		studioDeps: studioDependencies{
			sessionRepo:       config.Core.StudioSessionRepository,
			batchRepo:         config.Core.StudioBatchRepository,
			batchRunRepo:      config.Core.StudioBatchRunRepository,
			promptDiversifier: config.Shein.StudioPromptDiversifier,
			imageGenerator:    config.Shein.StudioImageGenerator,
			uploadStore:       config.Core.ImageUploadStore,
		},
		submission: submissionCollaborators{
			sheinSubmitLocks: submission.NewSubmitLockManager(),
		},
		adminDeps: adminDependencies{
			storeProfileRepo:  config.Core.StoreProfileRepository,
			aiCredentialStore: config.Core.AIClientCredentialStore,
		},
		submissionDeps: submissionDependencies{
			storeProfileRepo:            config.Core.StoreProfileRepository,
			sheinStoreCatalog:           config.Shein.SheinStoreCatalog,
			sheinAPIClientFactory:       config.Shein.SheinAPIClientFactory,
			sheinProductAPIBuilder:      config.Shein.SheinProductAPIBuilder,
			sheinImageAPIBuilder:        config.Shein.SheinImageAPIBuilder,
			sheinTranslateAPIBuilder:    config.Shein.SheinTranslateAPIBuilder,
			sheinContentOptimizer:       config.Shein.SheinContentOptimizer,
			sheinPublishWorkflowClient:  config.Workflow.SheinPublishWorkflowClient,
			sheinPublishWorkflowEnabled: config.Workflow.SheinPublishWorkflowEnabled,
		},
		workflowDeps: workflowDependencies{
			productService:         config.Core.ProductService,
			imageService:           config.Core.ImageService,
			assetRepository:        config.Assets.AssetRepository,
			assetRecipeResolver:    config.Assets.AssetRecipeResolver,
			assetBundleBuilder:     config.Assets.AssetBundleBuilder,
			assetGenerationService: config.Assets.AssetGenerationService,
			sheinContentOptimizer:  config.Shein.SheinContentOptimizer,
		},
		sheinSettings: defaultSettings,
	}
}
