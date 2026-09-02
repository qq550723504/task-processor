package httpapi

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/listingsubscription"
	worker "task-processor/internal/platform/workerpool"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
)

type Module struct {
	Handler                    RouteHandler
	ImageAgentWorkspaceHandler ImageAgentWorkspaceRouteHandler
	TaskLifecycleService       listingkit.TaskLifecycleService
	TaskRepository             listingkit.Repository
	StoreAccessValidator       listingkit.StoreAccessValidator
	StoreRepository            listingadmin.StoreRepository
	Pool                       worker.WorkerPool
	Closers                    []func() error
}

type ServiceBundle struct {
	TemporalWorkerService           TemporalWorkerService
	TaskRepository                  listingkit.Repository
	StoreRepository                 listingadmin.StoreRepository
	StoreStatisticsRepository       listingadmin.StoreStatisticsRepository
	DispatchEventRepository         listingadmin.DispatchEventRepository
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
	temporalWorkerService         TemporalWorkerService
	taskRepository                listingkit.Repository
	service                       moduleService
	sheinSyncRepository           listingkit.SheinSyncRepository
	sheinSyncService              listingkit.SheinSyncService
	sdsRetirementSheinSyncService listingkit.SheinSyncService
	sheinCandidateService         listingkit.SheinCandidateService
	sheinEnrollmentService        listingkit.SheinEnrollmentService
	handlerDependencies           listingkitapi.HandlerDependencies
	closers                       []func() error
}

type TemporalWorkerService interface {
	listingkit.SheinPublishActivityHostSource
	listingkit.LayerWorkflowActivityHostSource
}

type moduleService interface {
	listingkit.TaskLifecycleService
	listingkit.ChildTaskRetryService
	listingkit.SDSBaselineWarmService
	listingkit.StoreAdminService
	listingkit.UploadedImageService
	listingkit.InternalListingKitService
	listingkit.TaskSubmitterConfigurer
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

type AdminRepositories struct {
	Store                   listingadmin.StoreRepository
	StoreStatistics         listingadmin.StoreStatisticsRepository
	DispatchEvent           listingadmin.DispatchEventRepository
	ImportTask              listingadmin.ImportTaskRepository
	FilterRule              listingadmin.FilterRuleRepository
	ProfitRule              listingadmin.ProfitRuleRepository
	PricingRule             listingadmin.PricingRuleRepository
	OperationStrategy       listingadmin.OperationStrategyRepository
	ScheduledTaskConfig     listingadmin.ScheduledTaskConfigRepository
	SensitiveWord           listingadmin.SensitiveWordRepository
	GenerationTopicOverride listingadmin.GenerationTopicOverrideRepository
	GenerationTopicPolicy   listingadmin.GenerationTopicPolicyRepository
	ProductImportMapping    listingadmin.ProductImportMappingRepository
	Category                listingadmin.CategoryRepository
	ProductData             listingadmin.ProductDataRepository
}

type CoreRepositories struct {
	Task                  listingkit.Repository
	SheinSync             listingkit.SheinSyncRepository
	Subscription          listingsubscription.Repository
	GenerationUsageLedger listingsubscription.UsageLedger
	MemberInvitationAudit memberinvite.AuditRepository
	ApprovedAsset         listingkit.ApprovedAssetInventoryReader
	Review                reviewstore.Repository
	UploadedImage         listingkit.UploadedImageRepository
	StoreProfile          listingkit.StoreProfileRepository
	SheinResolutionCache  sheinpub.ResolutionCacheStore
}

type BuildServiceRepositories struct {
	Core  CoreRepositories
	Admin AdminRepositories
}

type BuildServiceHooks struct {
	SheinPricingPolicyBuilder         func(*config.Config) sheinpub.PricingPolicy
	ImageUploadStoreBuilder           func(*config.Config, *logrus.Logger) (listingkit.ImageUploadStore, error)
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
}

type BuildServiceInput struct {
	Config                    *config.Config
	Logger                    *logrus.Logger
	ProductSnapshotReader     listingkit.ProductSnapshotReader
	SDSSyncService            sdsusecase.Service
	SDSLoginStatusProvider    listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
	AICredentialStore         aiCredentialStore
	Repositories              BuildServiceRepositories
	Hooks                     BuildServiceHooks
}
