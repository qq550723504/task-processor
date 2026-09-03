package httpapi

import "task-processor/internal/listingkit"

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
		ProductSnapshotReader:     in.input.ProductSnapshotReader,
		SDSSyncService:            in.input.SDSSyncService,
		SDSLoginStatusProvider:    in.input.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider: in.input.SDSBaselineRemoteProvider,
		ImageUploadStore:          in.submit.assets.imageUploadStore,
		UploadedImageRepository:   in.repositories.uploadedImageRepository,
		StoreProfileRepository:    in.repositories.storeProfileRepository,
		AIClientCredentialStore:   in.input.AIClientCredentialStore,
		GenerationUsageLedger:     generationUsageSettlementDependency(in),
		GenerationUsageAdmission:  generationUsageAdmissionForConfig(in.input.Config),
	}
}

func generationUsageSettlementDependency(in buildListingKitServiceConfigInput) listingkit.GenerationUsageSettlement {
	if in.repositories == nil || in.repositories.subscriptionService == nil || !in.repositories.subscriptionService.HasUsageLedger() {
		return nil
	}
	return newSubscriptionGenerationUsage(in.repositories.subscriptionService)
}

func buildListingKitAssetDependencies(in buildListingKitServiceConfigInput) listingkit.ServiceAssetDependencies {
	return listingkit.ServiceAssetDependencies{
		ApprovedAssetInventoryReader: in.repositories.approvedAssetInventoryReader,
		Assembler:                    in.submit.assets.assembler,
		ReviewRepository:             in.repositories.reviewRepository,
	}
}

func buildListingKitSheinDependencies(in buildListingKitServiceConfigInput) listingkit.ServiceSheinDependencies {
	return listingkit.ServiceSheinDependencies{
		SheinStoreCatalog:          sheinListingStoreCatalog{repo: in.repositories.storeRepository},
		StoreAccessValidator:       listingAdminStoreAccessValidator{repo: in.repositories.storeRepository},
		SheinAPIClientFactory:      in.submit.shein.apiClientFactory,
		SheinCategoryResolver:      in.submit.shein.categoryResolver,
		SheinResolutionCacheStore:  in.repositories.resolutionCacheStore,
		SheinAttributeResolver:     in.submit.shein.attributeResolver,
		SheinSaleAttributeResolver: in.submit.shein.saleAttributeResolver,
		SheinSizeHeaderResolver:    in.submit.shein.sizeHeaderResolver,
		SheinPricingPolicy:         in.submit.shein.pricingPolicy,
		SheinProductAPIBuilder:     in.submit.shein.productAPIBuilder,
		SheinImageAPIBuilder:       in.submit.shein.imageAPIBuilder,
		SheinTranslateAPIBuilder:   in.submit.shein.translateAPIBuilder,
		SheinContentOptimizer:      in.submit.shein.contentOptimizer,
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
