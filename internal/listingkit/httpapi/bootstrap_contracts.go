package httpapi

import (
	"context"

	"github.com/sirupsen/logrus"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/listingsubscription"
	productenrich "task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
)

type Module struct {
	Handler              RouteHandler
	StudioSessionHandler listingkit.StudioSessionHandler
	Pool                 worker.WorkerPool
	Closers              []func() error
}

type ServiceBundle struct {
	TemporalWorkerService           TemporalWorkerService
	TaskRepository                  listingkit.Repository
	StudioAsyncJobRepository        listingkit.StudioAsyncJobRepository
	StoreRepository                 listingadmin.StoreRepository
	StoreStatisticsRepository       listingadmin.StoreStatisticsRepository
	ImportTaskRepository            listingadmin.ImportTaskRepository
	FilterRuleRepository            listingadmin.FilterRuleRepository
	ProfitRuleRepository            listingadmin.ProfitRuleRepository
	PricingRuleRepository           listingadmin.PricingRuleRepository
	OperationStrategyRepository     listingadmin.OperationStrategyRepository
	SensitiveWordRepository         listingadmin.SensitiveWordRepository
	GenerationTopicPolicyRepository listingadmin.GenerationTopicPolicyRepository
	ProductImportMappingRepository  listingadmin.ProductImportMappingRepository
	CategoryRepository              listingadmin.CategoryRepository
	ProductDataRepository           listingadmin.ProductDataRepository
	SubscriptionService             *listingsubscription.Service
	Closers                         []func() error

	runtime serviceBundleRuntime
}

type serviceBundleRuntime struct {
	temporalWorkerService  TemporalWorkerService
	taskRepository         listingkit.Repository
	service                moduleService
	sheinSyncRepository    listingkit.SheinSyncRepository
	sheinSyncService       listingkit.SheinSyncService
	sheinCandidateService  listingkit.SheinCandidateService
	sheinEnrollmentService listingkit.SheinEnrollmentService
	handlerDependencies    listingkitapi.HandlerDependencies
	closers                []func() error
}

type TemporalWorkerService interface {
	listingkit.SheinPublishActivityHostSource
	listingkit.LayerWorkflowActivityHostSource
}

type moduleService interface {
	listingkit.TaskLifecycleService
	listingkit.GenerationTaskService
	listingkit.StoreAdminService
	listingkit.StudioBatchRunService
	listingkit.StudioMediaService
	listingkit.InternalListingKitService
	listingkit.TaskSubmitterConfigurer
	listingkit.StudioSessionHandlerService
	listingkit.WorkflowClientConfigurer
	TemporalWorkerService
}

type aiCredentialStore interface {
	openaiclient.ClientConfigResolver
	SaveCredential(ctx context.Context, credential openaiclient.AIClientCredential) error
	GetCredential(ctx context.Context, tenantID, userID, clientName string) (*openaiclient.AIClientCredential, error)
}

type BuildModuleInput struct {
	ServiceInput                       BuildServiceInput
	ShouldStartTemporalWorkerInProcess bool
}

type AdminRepositoryBuilders struct {
	Store                   func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error)
	StoreStatistics         func(*config.Config, *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error)
	ImportTask              func(*config.Config, *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error)
	FilterRule              func(*config.Config, *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error)
	ProfitRule              func(*config.Config, *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error)
	PricingRule             func(*config.Config, *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error)
	OperationStrategy       func(*config.Config, *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error)
	SensitiveWord           func(*config.Config, *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error)
	GenerationTopicOverride func(*config.Config, *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error)
	GenerationTopicPolicy   func(*config.Config, *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error)
	ProductImportMapping    func(*config.Config, *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error)
	Category                func(*config.Config, *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error)
	ProductData             func(*config.Config, *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error)
}

type CoreRepositoryBuilders struct {
	Task                 func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error)
	StudioAsyncJob       func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error)
	StudioBatch          func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error)
	StudioBatchRun       func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error)
	SheinSync            func(*config.Config, *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error)
	Subscription         func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error)
	Asset                func(*config.Config, *logrus.Logger) (assetrepo.Repository, []func() error, error)
	Review               func(*config.Config, *logrus.Logger) (reviewstore.Repository, []func() error, error)
	StudioSession        func(*config.Config, *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error)
	UploadedImage        func(*config.Config, *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error)
	StoreProfile         func(*config.Config, *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error)
	SheinResolutionCache func(*config.Config, *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error)
}

type BuildServiceRepositories struct {
	Core  CoreRepositoryBuilders
	Admin AdminRepositoryBuilders
}

type BuildServiceHooks struct {
	SheinPricingPolicyBuilder         func(*config.Config) sheinpub.PricingPolicy
	ImageUploadStoreBuilder           func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore
	LegacyTenantResolverConfigurator  func(*config.Config, *logrus.Logger) (func() error, error)
	SheinCategoryLLMClientBuilder     func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinSaleAttributeLLMBuilder      func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinCategoryResolverBuilder      func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver
	SheinAttributeResolverBuilder     func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver
	SheinSaleAttributeResolverBuilder func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver
	SheinProductAPIBuilderFactory     func(listingadmin.StoreRepository) sheinpub.ProductAPIBuilder
	SheinImageAPIBuilderFactory       func(listingadmin.StoreRepository) sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilderFactory   func(listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder
	SheinAPIClientFactoryBuilder      func(listingadmin.StoreRepository) listingkit.SheinAPIClientFactory
	StudioImageGeneratorBuilder       func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator
	DefaultSheinStoreIDResolver       func([]int64) int64
	ConfigureZitadelAuth              func(config.ListingKitZitadelConfig)
	ConfigureAuthorization            func([]string, []string) error
}

type BuildServiceInput struct {
	Config                     *config.Config
	Logger                     *logrus.Logger
	ProductService             productenrich.ProductService
	ImageService               productimage.Service
	SDSSyncService             sdsusecase.Service
	SDSLoginStatusProvider     listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider  listingkit.SDSBaselineRemoteProvider
	ImageSubjectExtractor      productimage.SubjectExtractor
	ImageWhiteBackgroundRender productimage.WhiteBackgroundRenderer
	ImageSceneRenderer         productimage.SceneRenderer
	AICredentialStore          aiCredentialStore
	Repositories               BuildServiceRepositories
	Hooks                      BuildServiceHooks
}
