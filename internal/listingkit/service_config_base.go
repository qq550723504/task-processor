package listingkit

func newServiceBase(config *ServiceConfig, defaultSettings SheinSettings) *service {
	if config == nil {
		return &service{sheinSettings: defaultSettings}
	}
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
		sheinSettings:                  defaultSettings,
	}
}
