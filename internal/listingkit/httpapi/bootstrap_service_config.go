package httpapi

import (
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/listingkit"
)

type buildListingKitServiceConfigInput struct {
	input        BuildServiceInput
	repositories *builtRepositories
	submit       submitModule
}

func buildListingKitServiceConfig(in buildListingKitServiceConfigInput) *listingkit.ServiceConfig {
	return &listingkit.ServiceConfig{
		Core:     buildListingKitCoreDependencies(in),
		Assets:   buildListingKitAssetDependencies(in),
		Shein:    buildListingKitSheinDependencies(in),
		Workflow: buildListingKitWorkflowDependencies(),
		Health:   completeSettingsHealthProbesWithSubmitRuntime(buildSettingsHealthProbesFromConfig(in.input.Config), in.submit),
	}
}

func buildListingKitCoreDependencies(in buildListingKitServiceConfigInput) listingkit.ServiceCoreDependencies {
	return listingkit.ServiceCoreDependencies{
		Repository:                in.repositories.taskRepository,
		StudioSessionRepository:   in.repositories.studioSessionRepository,
		StudioBatchRepository:     in.repositories.studioBatchRepository,
		StudioBatchRunRepository:  in.repositories.studioBatchRunRepository,
		ProductService:            in.input.ProductService,
		ImageService:              in.input.ImageService,
		SDSSyncService:            in.input.SDSSyncService,
		SDSLoginStatusProvider:    in.input.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider: in.input.SDSBaselineRemoteProvider,
		ImageUploadStore:          in.submit.assets.imageUploadStore,
		UploadedImageRepository:   in.repositories.uploadedImageRepository,
		StoreProfileRepository:    in.repositories.storeProfileRepository,
		AIClientCredentialStore:   in.input.AICredentialStore,
	}
}

func buildListingKitAssetDependencies(in buildListingKitServiceConfigInput) listingkit.ServiceAssetDependencies {
	return listingkit.ServiceAssetDependencies{
		Assembler:           in.submit.assets.assembler,
		AssetRepository:     in.repositories.assetRepository,
		ReviewRepository:    in.repositories.reviewRepository,
		AssetRecipeResolver: assetrecipe.NewStaticResolver(),
		AssetBundleBuilder:  assetbundle.NewBuilder(),
		AssetGenerationService: assetgeneration.NewService(assetgeneration.Config{
			SubjectExtractor:        in.input.ImageSubjectExtractor,
			WhiteBackgroundRenderer: in.input.ImageWhiteBackgroundRender,
			DeferredRenderer:        assetgeneration.NewProductImageDeferredRenderer(in.input.ImageSceneRenderer),
		}),
	}
}

func buildListingKitSheinDependencies(in buildListingKitServiceConfigInput) listingkit.ServiceSheinDependencies {
	return listingkit.ServiceSheinDependencies{
		SheinDefaultStoreID:        in.submit.shein.defaultStoreID,
		SheinStoreCatalog:          sheinManagementStoreCatalog{repo: in.repositories.storeRepository},
		SheinAPIClientFactory:      in.submit.shein.apiClientFactory,
		SheinCategoryResolver:      in.submit.shein.categoryResolver,
		SheinResolutionCacheStore:  in.repositories.resolutionCacheStore,
		SheinAttributeResolver:     in.submit.shein.attributeResolver,
		SheinSaleAttributeResolver: in.submit.shein.saleAttributeResolver,
		SheinPricingPolicy:         in.submit.shein.pricingPolicy,
		SheinProductAPIBuilder:     in.submit.shein.productAPIBuilder,
		SheinImageAPIBuilder:       in.submit.shein.imageAPIBuilder,
		SheinTranslateAPIBuilder:   in.submit.shein.translateAPIBuilder,
		SheinContentOptimizer:      in.submit.shein.contentOptimizer,
		StudioPromptDiversifier:    in.submit.shein.contentOptimizer,
		StudioImageGenerator:       in.submit.studio.imageGenerator,
	}
}

func buildListingKitWorkflowDependencies() listingkit.ServiceWorkflowDependencies {
	return listingkit.ServiceWorkflowDependencies{
		SheinPublishWorkflowClient:     nil,
		SheinPublishWorkflowEnabled:    false,
		StandardProductWorkflowClient:  nil,
		StandardProductWorkflowEnabled: false,
		PlatformAdaptWorkflowClient:    nil,
		PlatformAdaptWorkflowEnabled:   false,
	}
}
