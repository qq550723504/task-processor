package consumer

import (
	"context"

	"task-processor/internal/app/ports"
	"task-processor/internal/app/runner"
	apptask "task-processor/internal/app/task"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/product"
	temupricingruntime "task-processor/internal/temu/pricing"
	temusyncruntime "task-processor/internal/temu/sync"

	"github.com/sirupsen/logrus"
)

type SchedulerFactoryRuntime interface {
	runner.SchedulerRuntimeProvider
	GetStoreAPI() listingadmin.StoreAPI
	GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error)
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetPricingRuleClient() listingadmin.PricingRuleAPI
	GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI
	GetInventoryRecordAPI() listingadmin.InventoryRecordAPI
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	PricingRuntime() temupricingruntime.PricingRuntime
	SyncRuntime() temusyncruntime.ServiceRuntime
	GetRuntimeStoreService() listingruntime.StoreService
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
}

type ProcessorRuntime interface {
	SchedulerFactoryRuntime
	GetDailyListingCountClient() listingadmin.DailyListingCountAPI
	GetFilterRuleClient() listingadmin.FilterRuleAPI
	GetProductImportMappingClient() listingadmin.ProductImportMappingAPI
	GetProfitRuleClient() listingadmin.ProfitRuleAPI
	GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository
	GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
	GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error)
	GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error)
	DeleteSheinStoreCookie(storeID int64) (bool, error)
	GetImageDownloader() interface {
		DownloadImage(url string) ([]byte, error)
	}
	SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
	RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error)
	FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error)
	CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error)
	UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error
	GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error)
}

type ListingRuntimeImportTaskRepository interface {
	ProcessingTimeoutRepository
	StaleQueuedRepository
}

type SchedulerResources struct {
	runtime        runner.SchedulerRuntimeProvider
	factoryRuntime SchedulerFactoryRuntime
	crawlSource    ports.CrawlSource
}

func NewSchedulerResources(runtime runner.SchedulerRuntimeProvider, factoryRuntime SchedulerFactoryRuntime, crawlSource ports.CrawlSource) SchedulerResources {
	return SchedulerResources{
		runtime:        runtime,
		factoryRuntime: factoryRuntime,
		crawlSource:    crawlSource,
	}
}

func (r SchedulerResources) Runtime() runner.SchedulerRuntimeProvider {
	return r.runtime
}

func (r SchedulerResources) FactoryRuntime() SchedulerFactoryRuntime {
	return r.factoryRuntime
}

func (r SchedulerResources) CrawlSource() ports.CrawlSource {
	return r.crawlSource
}

type SharedResources struct {
	listingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository
	storeAPI                           listingadmin.StoreAPI
	processorRuntime                   ProcessorRuntime
	productFetcher                     appfetcher.ProductFetcher
	scheduler                          SchedulerResources
}

type SharedResourcesInput struct {
	ListingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository
	StoreAPI                           listingadmin.StoreAPI
	ProcessorRuntime                   ProcessorRuntime
	ProductFetcher                     appfetcher.ProductFetcher
	Scheduler                          SchedulerResources
}

func NewSharedResources(input SharedResourcesInput) SharedResources {
	return SharedResources{
		listingRuntimeImportTaskRepository: input.ListingRuntimeImportTaskRepository,
		storeAPI:                           input.StoreAPI,
		processorRuntime:                   input.ProcessorRuntime,
		productFetcher:                     input.ProductFetcher,
		scheduler:                          input.Scheduler,
	}
}

func (r SharedResources) ListingRuntimeImportTaskRepository() ListingRuntimeImportTaskRepository {
	return r.listingRuntimeImportTaskRepository
}

func (r SharedResources) StoreAPI() listingadmin.StoreAPI {
	return r.storeAPI
}

func (r SharedResources) ProcessorRuntime() ProcessorRuntime {
	return r.processorRuntime
}

func (r SharedResources) ProductFetcher() appfetcher.ProductFetcher {
	return r.productFetcher
}

func (r SharedResources) Scheduler() SchedulerResources {
	return r.scheduler
}

type PlatformRuntimeResources struct {
	listingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository
	storeAPI                           listingadmin.StoreAPI
	processorRuntime                   ProcessorRuntime
	productFetcher                     appfetcher.ProductFetcher
	scheduler                          SchedulerResources
}

func NewPlatformRuntimeResources(resources SharedResources) PlatformRuntimeResources {
	return PlatformRuntimeResources{
		listingRuntimeImportTaskRepository: resources.ListingRuntimeImportTaskRepository(),
		storeAPI:                           resources.StoreAPI(),
		processorRuntime:                   resources.ProcessorRuntime(),
		productFetcher:                     resources.ProductFetcher(),
		scheduler:                          resources.Scheduler(),
	}
}

type SharedResourceNeeds struct {
	NeedAmazonCrawler bool
}

type SchedulerDependenciesBuilder func(
	schedulerRuntime SchedulerFactoryRuntime,
	cfg *config.Config,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies

type PlatformRuntimeServices interface {
	StoreAssignmentRuntime
	StaticStoreGuardRuntime
	SchedulerServiceRuntime
	TaskRecoveryRuntime
	AutoShardRuntime
	GetClient() *rabbitmq.Client
}

type PlatformRegistrationServices interface {
	PlatformRuntimeServices
	ProcessorRegistrar
	LogDistributedFetchingAvailability(*logrus.Logger)
}

type SchedulerServiceRuntime interface {
	GetClient() *rabbitmq.Client
	SetSchedulerService(SchedulerService)
}

type TaskRecoveryRuntime interface {
	SetProcessingTimeoutWatchdog(SchedulerService)
	SetStaleQueuedWatchdog(SchedulerService)
}

type AutoShardRuntime interface {
	SetAutoShardService(AutoShardService)
}

type StoreAssignmentRuntime interface {
	SetStoreAssignmentProvider(StoreAssignmentProvider)
}

type StaticStoreGuardRuntime interface {
	SetStoreComponents(listingadmin.StoreAPI, []int64, *apptask.DeduplicationManager)
}

type PlatformRuntimeContext struct {
	config                             *config.Config
	logger                             *logrus.Logger
	listingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository
	storeAPI                           listingadmin.StoreAPI
	processorRuntime                   ProcessorRuntime
	productFetcher                     appfetcher.ProductFetcher
	schedulerRuntime                   runner.SchedulerRuntimeProvider
	schedulerFactoryRuntime            SchedulerFactoryRuntime
	crawlSource                        ports.CrawlSource
	schedulerBuilder                   SchedulerDependenciesBuilder
	runtimeServices                    PlatformRuntimeServices
}

type PlatformRuntimeContextInput struct {
	Config           *config.Config
	Logger           *logrus.Logger
	Resources        PlatformRuntimeResources
	Services         PlatformRuntimeServices
	SchedulerBuilder SchedulerDependenciesBuilder
}

func BuildPlatformRuntimeContext(input PlatformRuntimeContextInput) PlatformRuntimeContext {
	schedulerResources := input.Resources.scheduler
	return PlatformRuntimeContext{
		config:                             input.Config,
		logger:                             input.Logger,
		listingRuntimeImportTaskRepository: input.Resources.listingRuntimeImportTaskRepository,
		storeAPI:                           input.Resources.storeAPI,
		processorRuntime:                   input.Resources.processorRuntime,
		productFetcher:                     input.Resources.productFetcher,
		schedulerRuntime:                   schedulerResources.Runtime(),
		schedulerFactoryRuntime:            schedulerResources.FactoryRuntime(),
		crawlSource:                        schedulerResources.CrawlSource(),
		schedulerBuilder:                   input.SchedulerBuilder,
		runtimeServices:                    input.Services,
	}
}

func (rt PlatformRuntimeContext) RabbitMQClient() *rabbitmq.Client {
	return runtimeRabbitMQClient(rt.runtimeServices)
}

func (rt PlatformRuntimeContext) Config() *config.Config {
	return rt.config
}

func (rt PlatformRuntimeContext) Logger() *logrus.Logger {
	return rt.logger
}

func runtimeRabbitMQClient(services PlatformRuntimeServices) *rabbitmq.Client {
	if services == nil {
		return nil
	}
	return services.GetClient()
}

func (rt PlatformRuntimeContext) StoreAssignmentRuntime() StoreAssignmentRuntime {
	if rt.runtimeServices == nil {
		return nil
	}
	return rt.runtimeServices
}

func (rt PlatformRuntimeContext) StaticStoreGuardRuntime() StaticStoreGuardRuntime {
	if rt.runtimeServices == nil {
		return nil
	}
	return rt.runtimeServices
}

func (rt PlatformRuntimeContext) SchedulerServiceRuntime() SchedulerServiceRuntime {
	if rt.runtimeServices == nil {
		return nil
	}
	return rt.runtimeServices
}

func (rt PlatformRuntimeContext) TaskRecoveryRuntime() TaskRecoveryRuntime {
	if rt.runtimeServices == nil {
		return nil
	}
	return rt.runtimeServices
}

func (rt PlatformRuntimeContext) AutoShardRuntime() AutoShardRuntime {
	if rt.runtimeServices == nil {
		return nil
	}
	return rt.runtimeServices
}

func (rt PlatformRuntimeContext) StoreAPI() listingadmin.StoreAPI {
	return rt.storeAPI
}

func (rt PlatformRuntimeContext) ProcessorRuntime() ProcessorRuntime {
	return rt.processorRuntime
}

func (rt PlatformRuntimeContext) ProductFetcher() appfetcher.ProductFetcher {
	return rt.productFetcher
}

func (rt PlatformRuntimeContext) ListingRuntimeImportTaskRepository() ListingRuntimeImportTaskRepository {
	return rt.listingRuntimeImportTaskRepository
}

func (rt PlatformRuntimeContext) SchedulerRuntime() runner.SchedulerRuntimeProvider {
	return rt.schedulerRuntime
}

func (rt PlatformRuntimeContext) HasSchedulerDependenciesBuilder() bool {
	return rt.schedulerBuilder != nil
}

func (rt PlatformRuntimeContext) BuildSchedulerDependencies(rabbitmqClient *rabbitmq.Client) runner.SchedulerDependencies {
	if rt.schedulerBuilder == nil {
		return runner.SchedulerDependencies{}
	}
	return rt.schedulerBuilder(rt.schedulerFactoryRuntime, rt.config, rt.crawlSource, rabbitmqClient)
}

type PlatformModule interface {
	Name() string
	Enabled(cfg *config.Config) bool
	NeedsAmazon(cfg *config.Config) bool
	RegisterConsumer(ctx context.Context, rt PlatformRuntimeContext, registry ProcessorRegistrar) error
	ConfigureListingRuntime(ctx context.Context, rt PlatformRuntimeContext) error
}

type ProcessorRegistrar interface {
	RegisterProcessor(platform string, processor worker.Processor) error
}
