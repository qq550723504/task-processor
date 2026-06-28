package consumer

import (
	"context"

	"task-processor/internal/app/ports"
	"task-processor/internal/app/runner"
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

type SharedResources struct {
	ListingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository
	StoreAPI                           listingadmin.StoreAPI
	SchedulerRuntime                   runner.SchedulerRuntimeProvider
	SchedulerFactoryRuntime            SchedulerFactoryRuntime
	ProcessorRuntime                   ProcessorRuntime
	CrawlSource                        ports.CrawlSource
	ProductFetcher                     appfetcher.ProductFetcher
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

type PlatformRuntimeContext struct {
	Config                             *config.Config
	Logger                             *logrus.Logger
	ListingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository
	StoreAPI                           listingadmin.StoreAPI
	SchedulerRuntime                   runner.SchedulerRuntimeProvider
	SchedulerFactoryRuntime            SchedulerFactoryRuntime
	ProcessorRuntime                   ProcessorRuntime
	CrawlSource                        ports.CrawlSource
	ProductFetcher                     appfetcher.ProductFetcher
	RabbitMQClient                     *rabbitmq.Client
	ServiceManager                     *ServiceManager
	SchedulerBuilder                   SchedulerDependenciesBuilder
}

type PlatformRuntimeContextInput struct {
	Config           *config.Config
	Logger           *logrus.Logger
	Resources        *SharedResources
	ServiceManager   *ServiceManager
	SchedulerBuilder SchedulerDependenciesBuilder
}

func BuildPlatformRuntimeContext(input PlatformRuntimeContextInput) PlatformRuntimeContext {
	resources := sharedResourcesValue(input.Resources)
	return PlatformRuntimeContext{
		Config:                             input.Config,
		Logger:                             input.Logger,
		ListingRuntimeImportTaskRepository: resources.ListingRuntimeImportTaskRepository,
		StoreAPI:                           resources.StoreAPI,
		SchedulerRuntime:                   resources.SchedulerRuntime,
		SchedulerFactoryRuntime:            resources.SchedulerFactoryRuntime,
		ProcessorRuntime:                   resources.ProcessorRuntime,
		CrawlSource:                        resources.CrawlSource,
		ProductFetcher:                     resources.ProductFetcher,
		RabbitMQClient:                     runtimeRabbitMQClient(input.ServiceManager),
		ServiceManager:                     input.ServiceManager,
		SchedulerBuilder:                   input.SchedulerBuilder,
	}
}

func sharedResourcesValue(resources *SharedResources) SharedResources {
	if resources == nil {
		return SharedResources{}
	}
	return *resources
}

func runtimeRabbitMQClient(serviceManager *ServiceManager) *rabbitmq.Client {
	if serviceManager == nil {
		return nil
	}
	return serviceManager.GetClient()
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
