package listingkit

import (
	"sync"

	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingkit/reviewstore"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
)

type service struct {
	repo             Repository
	task             taskCollaborators
	taskDeps         taskDependencies
	studio           studioCollaborators
	studioDeps       studioDependencies
	admin            adminCollaborators
	adminDeps        adminDependencies
	submission       submissionCollaborators
	submissionDeps   submissionDependencies
	sheinSharedDeps  sheinSharedDependencies
	workflowDeps     workflowDependencies
	sheinRuntimeDeps sheinRuntimeDependencies
	supportDeps      supportDependencies
	healthProbes     SettingsHealthProbes
	sheinSettingsMu  sync.RWMutex
	sheinSettings    SheinSettings
}

type ServiceCoreDependencies struct {
	Repository                    Repository
	StudioSessionRepository       StudioSessionRepository
	StudioBatchRepository         StudioBatchRepository
	StudioBatchRunRepository      StudioBatchRunRepository
	StudioBatchTaskLinkRepository StudioBatchTaskLinkRepository
	ProductService                ProductService
	ImageService                  ImageService
	SDSSyncService                sdsusecase.Service
	SDSLoginStatusProvider        SDSLoginStatusProvider
	SDSBaselineRemoteProvider     SDSBaselineRemoteProvider
	ImageUploadStore              ImageUploadStore
	UploadedImageRepository       UploadedImageRepository
	StoreProfileRepository        StoreProfileRepository
	TaskSubmitter                 TaskSubmitter
	AIClientCredentialStore       AIClientCredentialStore
}

type ServiceAssetDependencies struct {
	Assembler              Assembler
	AssetRepository        assetrepo.Repository
	ReviewRepository       reviewstore.Repository
	AssetRecipeResolver    assetrecipe.Resolver
	AssetBundleBuilder     assetbundle.Builder
	AssetGenerationService assetgeneration.Service
}

type ServiceSheinDependencies struct {
	SheinDefaultStoreID        int64
	SheinStoreCatalog          SheinStoreCatalog
	SheinAPIClientFactory      SheinAPIClientFactory
	SheinCategoryResolver      sheinpub.CategoryResolver
	SheinResolutionCacheStore  sheinpub.ResolutionCacheStore
	SheinAttributeResolver     sheinpub.AttributeResolver
	SheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	SheinSizeHeaderResolver    sheinpub.SizeAttributeHeaderResolver
	SheinPricingPolicy         sheinpub.PricingPolicy
	SheinProductAPIBuilder     sheinpub.ProductAPIBuilder
	SheinImageAPIBuilder       sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilder   sheinpub.TranslateAPIBuilder
	SheinContentOptimizer      AIChatCompleter
	StudioPromptDiversifier    AIChatCompleter
	StudioImageGenerator       AIImageGenerator
}

type ServiceWorkflowDependencies struct {
	SheinPublishWorkflowClient     SheinPublishWorkflowClient
	SheinPublishWorkflowEnabled    bool
	StandardProductWorkflowClient  StandardProductWorkflowClient
	StandardProductWorkflowEnabled bool
	PlatformAdaptWorkflowClient    PlatformAdaptWorkflowClient
	PlatformAdaptWorkflowEnabled   bool
}

type ServiceConfig struct {
	Core     ServiceCoreDependencies
	Assets   ServiceAssetDependencies
	Shein    ServiceSheinDependencies
	Workflow ServiceWorkflowDependencies
	Health   SettingsHealthProbes
}
