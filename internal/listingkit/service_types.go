package listingkit

import (
	"sync"

	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit/reviewstore"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
)

type service struct {
	repo                           Repository
	taskLifecycle                  *taskLifecycleService
	taskGeneration                 *taskGenerationService
	taskRevision                   *taskRevisionService
	taskPreview                    *taskPreviewService
	taskExport                     *taskExportService
	sdsBaseline                    *sdsBaselineService
	task                           taskCollaborators
	taskDeps                       taskDependencies
	taskStudioSession              *taskStudioSessionService
	taskStudioBatchDraft           *taskStudioBatchDraftService
	studioBatchGeneration          *studioBatchGenerationService
	taskStudioBatch                *taskStudioBatchService
	studioBatchRunExecutor         *taskStudioBatchRunExecutor
	studioBatchRunCoordinator      *studioBatchRunCoordinator
	taskStudioBatchRun             *taskStudioBatchRunService
	taskStudioMedia                *taskStudioMediaService
	studio                         studioCollaborators
	studioDeps                     studioDependencies
	settingsAdmin                  *settingsAdminService
	sheinAdmin                     *sheinAdminService
	admin                          adminCollaborators
	adminDeps                      adminDependencies
	submission                     submissionCollaborators
	submissionDeps                 submissionDependencies
	workflowDeps                   workflowDependencies
	sheinRuntimeDeps               sheinRuntimeDependencies
	supportDeps                    supportDependencies
	studioSessionRepo              StudioSessionRepository
	studioBatchRepo                StudioBatchRepository
	studioBatchRunRepo             StudioBatchRunRepository
	productSvc                     ProductService
	imageSvc                       ImageService
	sdsSyncSvc                     sdsusecase.Service
	sdsLoginStatusProvider         SDSLoginStatusProvider
	sdsBaselineRemoteProvider      SDSBaselineRemoteProvider
	uploadStore                    ImageUploadStore
	uploadedImageRepo              UploadedImageRepository
	assembler                      Assembler
	sheinCategoryResolver          sheinpub.CategoryResolver
	sheinResolutionCacheStore      sheinpub.ResolutionCacheStore
	sheinStoreCatalog              SheinStoreCatalog
	sheinAPIClientFactory          SheinAPIClientFactory
	sheinAttributeResolver         sheinpub.AttributeResolver
	sheinSaleAttributeResolver     sheinpub.SaleAttributeResolver
	sheinPricingPolicy             sheinpub.PricingPolicy
	sheinProductAPIBuilder         sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder           sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder       sheinpub.TranslateAPIBuilder
	sheinContentOptimizer          openaiclient.ChatCompleter
	studioPromptDiversifier        openaiclient.ChatCompleter
	studioImageGenerator           openaiclient.ImageGenerator
	aiCredentialStore              AIClientCredentialStore
	assetRepo                      assetrepo.Repository
	reviewRepo                     reviewstore.Repository
	assetRecipeResolver            assetrecipe.Resolver
	assetBundleBuilder             assetbundle.Builder
	assetGenerator                 assetgeneration.Service
	taskSubmitter                  TaskSubmitter
	sheinPublishWorkflowClient     SheinPublishWorkflowClient
	sheinPublishWorkflowEnabled    bool
	standardProductWorkflowClient  StandardProductWorkflowClient
	standardProductWorkflowEnabled bool
	platformAdaptWorkflowClient    PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled   bool
	storeProfileRepo               StoreProfileRepository
	sheinSettingsMu                sync.RWMutex
	sheinSettings                  SheinSettings
}

type ServiceCoreDependencies struct {
	Repository                Repository
	StudioSessionRepository   StudioSessionRepository
	StudioBatchRepository     StudioBatchRepository
	StudioBatchRunRepository  StudioBatchRunRepository
	ProductService            ProductService
	ImageService              ImageService
	SDSSyncService            sdsusecase.Service
	SDSLoginStatusProvider    SDSLoginStatusProvider
	SDSBaselineRemoteProvider SDSBaselineRemoteProvider
	ImageUploadStore          ImageUploadStore
	UploadedImageRepository   UploadedImageRepository
	StoreProfileRepository    StoreProfileRepository
	TaskSubmitter             TaskSubmitter
	AIClientCredentialStore   AIClientCredentialStore
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
	SheinPricingPolicy         sheinpub.PricingPolicy
	SheinProductAPIBuilder     sheinpub.ProductAPIBuilder
	SheinImageAPIBuilder       sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilder   sheinpub.TranslateAPIBuilder
	SheinContentOptimizer      openaiclient.ChatCompleter
	StudioPromptDiversifier    openaiclient.ChatCompleter
	StudioImageGenerator       openaiclient.ImageGenerator
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
}
