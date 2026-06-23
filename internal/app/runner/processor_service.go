package runner

import (
	"context"

	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/product"
	temupricingruntime "task-processor/internal/temu/pricing"
	temusyncruntime "task-processor/internal/temu/sync"

	"github.com/sirupsen/logrus"
)

type crawlSource = ports.CrawlSource
type CrawlSource = ports.CrawlSource
type rawJSONDataClientProvider = product.RawJsonDataClient

type processorRuntimeProvider interface {
	GetDailyListingCountClient() *management.DailyListingCountAPIClient
	GetStoreClient() *management.StoreAPIClient
	GetFilterRuleClient() *management.FilterRuleAPIClient
	GetProductImportMappingClient() *management.ProductImportMappingAPIClient
	GetProfitRuleClient() *management.ProfitRuleAPIClient
	GetRuntimeStoreService() listingruntime.StoreService
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository
	GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
	DeleteSheinStoreCookie(storeID int64) (bool, error)
	SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
	GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error)
	GetTaskStatus(taskID int64) (*managementapi.TaskStatusRespDTO, error)
	GetImageDownloader() *management.ImageDownloader
	RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error)
	FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error)
	CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error)
	UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error
	GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error)
}

type schedulerFactoryRuntimeProvider interface {
	SchedulerRuntimeProvider
	GetStoreClient() *management.StoreAPIClient
	GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error)
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetStoreAPI() managementapi.StoreAPI
	GetPricingRuleClient() managementapi.PricingRuleAPI
	GetProductImportMappingAPI() managementapi.ProductImportMappingAPI
	GetInventoryRecordAPI() managementapi.InventoryRecordAPI
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	PricingRuntime() temupricingruntime.ManagementRuntime
	SyncRuntime() temusyncruntime.ServiceRuntime
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
}

type ProcessorService interface {
	StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error
	StopProcessors() error
	GetStatus() map[string]any
}

func NewProcessorServiceWithCreators(
	logger *logrus.Logger,
	managementClient *management.ClientManager,
	rawJSONDataClient product.RawJsonDataClient,
	processorRuntime processorRuntimeProvider,
	schedulerRuntime SchedulerRuntimeProvider,
	schedulerFactoryRuntime schedulerFactoryRuntimeProvider,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
	deps ProcessorDependencies,
) ProcessorService {
	deps = normalizeProcessorDependencies(deps)

	return &processorServiceImpl{
		logger:                  logger,
		lifecycleManager:        lifecycle.NewLifecycleManager(logger),
		managementClient:        managementClient,
		rawJSONDataClient:       rawJSONDataClient,
		processorRuntime:        processorRuntime,
		schedulerRuntime:        schedulerRuntime,
		schedulerFactoryRuntime: schedulerFactoryRuntime,
		crawlSource:             crawlSource,
		rabbitmqClient:          rabbitmqClient,
		temuProcessorCreator:    deps.TemuProcessorCreator,
		sheinProcessorCreator:   deps.SheinProcessorCreator,
	}
}

func normalizeProcessorDependencies(deps ProcessorDependencies) ProcessorDependencies {
	defaultDeps := buildProcessorDependencies()
	if deps.TemuProcessorCreator == nil {
		deps.TemuProcessorCreator = defaultDeps.TemuProcessorCreator
	}
	if deps.SheinProcessorCreator == nil {
		deps.SheinProcessorCreator = defaultDeps.SheinProcessorCreator
	}

	return deps
}
