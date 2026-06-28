package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	platformtask "task-processor/internal/platformtask"
	appscheduler "task-processor/internal/scheduler"
	temuapi "task-processor/internal/temu/api"
	"task-processor/internal/temu/api/client"
	schedulerservice "task-processor/internal/temu/sync"
)

type TaskBuilder func(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error)
type PricingServiceBuilder func(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.AutoPricingService, error)
type ProductSyncServiceBuilder func(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.ProductSyncService, error)
type InventoryServiceBuilder func(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.InventorySyncService, error)
type ActivityServiceBuilder func(config appscheduler.TaskConfig, factory *TemuTaskFactory) (ActivityService, error)

type Dependencies struct {
	ClientManager             *client.APIClientManager
	FetcherBuilder            platformbase.ProductFetcherBuilder
	PricingServiceBuilder     PricingServiceBuilder
	ProductSyncServiceBuilder ProductSyncServiceBuilder
	InventoryServiceBuilder   InventoryServiceBuilder
	ActivityServiceBuilder    ActivityServiceBuilder
	PricingTaskBuilder        TaskBuilder
	ProductSyncTaskBuilder    TaskBuilder
	InventoryTaskBuilder      TaskBuilder
	ActivityTaskBuilder       TaskBuilder
}

type TemuTaskFactory struct {
	*platformbase.BaseFactory
	runtime                   runtime
	clientManager             *client.APIClientManager
	rabbitmqClient            *rabbitmq.Client
	fetcherBuilder            platformbase.ProductFetcherBuilder
	pricingServiceBuilder     PricingServiceBuilder
	productSyncServiceBuilder ProductSyncServiceBuilder
	inventoryServiceBuilder   InventoryServiceBuilder
	activityServiceBuilder    ActivityServiceBuilder
	pricingTaskBuilder        TaskBuilder
	productSyncTaskBuilder    TaskBuilder
	inventoryTaskBuilder      TaskBuilder
	activityTaskBuilder       TaskBuilder
}

func NewTemuTaskFactory(
	schedulerRuntime SchedulerRuntime,
	crawlSource ports.CrawlSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) *TemuTaskFactory {
	return NewTemuTaskFactoryWithDependencies(
		schedulerRuntime,
		crawlSource,
		amazonConfig,
		monitorConfig,
		rabbitmqClient,
		Dependencies{
			ClientManager:  client.NewAPIClientManager(schedulerRuntime),
			FetcherBuilder: platformbase.NewDefaultProductFetcherBuilder(),
		},
	)
}

func NewTemuTaskFactoryWithFetcherBuilder(
	schedulerRuntime SchedulerRuntime,
	fetcherBuilder platformbase.ProductFetcherBuilder,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) *TemuTaskFactory {
	return NewTemuTaskFactoryWithDependencies(
		schedulerRuntime,
		nil,
		amazonConfig,
		monitorConfig,
		rabbitmqClient,
		Dependencies{
			ClientManager:  client.NewAPIClientManager(schedulerRuntime),
			FetcherBuilder: fetcherBuilder,
		},
	)
}

func NewTemuTaskFactoryWithDependencies(
	schedulerRuntime SchedulerRuntime,
	crawlSource ports.CrawlSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
	deps Dependencies,
) *TemuTaskFactory {
	_ = crawlSource
	runtime := schedulerRuntime
	if runtime == nil {
		return nil
	}
	baseFactory := platformbase.NewBaseFactory(platformbase.BaseFactoryConfig{
		Platform:       "TEMU",
		Runtime:        runtime,
		FetcherBuilder: deps.FetcherBuilder,
		AmazonConfig:   amazonConfig,
		MonitorConfig:  monitorConfig,
	})

	factory := &TemuTaskFactory{
		BaseFactory:    baseFactory,
		runtime:        runtime,
		clientManager:  deps.ClientManager,
		rabbitmqClient: rabbitmqClient,
		fetcherBuilder: baseFactory.GetFetcherBuilder(),
	}

	if factory.clientManager == nil {
		factory.clientManager = client.NewAPIClientManager(runtime)
	}
	factory.pricingServiceBuilder = deps.PricingServiceBuilder
	if factory.pricingServiceBuilder == nil {
		factory.pricingServiceBuilder = defaultBuildTemuPricingService
	}
	factory.productSyncServiceBuilder = deps.ProductSyncServiceBuilder
	if factory.productSyncServiceBuilder == nil {
		factory.productSyncServiceBuilder = defaultBuildTemuProductSyncService
	}
	factory.inventoryServiceBuilder = deps.InventoryServiceBuilder
	if factory.inventoryServiceBuilder == nil {
		factory.inventoryServiceBuilder = defaultBuildTemuInventoryService
	}
	factory.activityServiceBuilder = deps.ActivityServiceBuilder
	if factory.activityServiceBuilder == nil {
		factory.activityServiceBuilder = defaultBuildTemuActivityService
	}
	factory.pricingTaskBuilder = deps.PricingTaskBuilder
	if factory.pricingTaskBuilder == nil {
		factory.pricingTaskBuilder = defaultBuildTemuPricingTask
	}
	factory.productSyncTaskBuilder = deps.ProductSyncTaskBuilder
	if factory.productSyncTaskBuilder == nil {
		factory.productSyncTaskBuilder = defaultBuildTemuProductSyncTask
	}
	factory.inventoryTaskBuilder = deps.InventoryTaskBuilder
	if factory.inventoryTaskBuilder == nil {
		factory.inventoryTaskBuilder = defaultBuildTemuInventoryTask
	}
	factory.activityTaskBuilder = deps.ActivityTaskBuilder
	if factory.activityTaskBuilder == nil {
		factory.activityTaskBuilder = defaultBuildTemuActivityTask
	}

	return factory
}

func (f *TemuTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	if err := f.ValidatePlatform(config); err != nil {
		return nil, err
	}
	if err := f.ValidateTaskType(config.TaskType); err != nil {
		return nil, err
	}

	switch config.TaskType {
	case appscheduler.TaskTypePricing:
		return f.pricingTaskBuilder(ctx, config, f)
	case appscheduler.TaskTypeProductSync:
		return f.productSyncTaskBuilder(ctx, config, f)
	case appscheduler.TaskTypeInventory:
		return f.inventoryTaskBuilder(ctx, config, f)
	case appscheduler.TaskTypeActivity:
		return f.activityTaskBuilder(ctx, config, f)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", config.TaskType)
	}
}

func defaultBuildTemuPricingTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
	pricingService, err := factory.pricingServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build TEMU pricing service: %w", err)
	}
	return NewPricingTask(ctx, config, factory.runtime, pricingService), nil
}

func defaultBuildTemuPricingService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.AutoPricingService, error) {
	apiClient := temuapi.NewAPIClient(config.StoreID, factory.runtime)
	if apiClient == nil {
		return nil, fmt.Errorf("create TEMU API client")
	}
	return NewTemuAutoPricingAdapter(apiClient, factory.runtime.PricingRuntime()), nil
}

func defaultBuildTemuProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
	syncService, err := factory.productSyncServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build TEMU product sync service: %w", err)
	}
	return NewProductSyncTask(ctx, config, syncService), nil
}

func defaultBuildTemuProductSyncService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.ProductSyncService, error) {
	apiClient, err := factory.clientManager.GetClient(config.TenantID, config.StoreID)
	if err != nil {
		return nil, fmt.Errorf("get TEMU API client: %w", err)
	}

	productAPI := temuapi.NewProductAPI(apiClient, logger.GetGlobalLogger("TemuProductAPI"))
	skuQueryAPI := temuapi.NewQueryAPI(apiClient, logger.GetGlobalLogger("TemuSkuQueryAPI"))
	mappingClient := factory.runtime.GetProductImportMappingAPI()
	storeAPI := factory.runtime.GetStoreAPI()

	syncConfig := &schedulerservice.ProductSyncConfig{
		PageSize:        100,
		MaxPages:        0,
		Language:        "en",
		IncludeInactive: false,
	}

	syncService := schedulerservice.NewProductSyncService(
		factory.runtime.SyncRuntime(),
		productAPI,
		skuQueryAPI,
		mappingClient,
		storeAPI,
		syncConfig,
	)

	return newProductSyncServiceAdapter(syncService), nil
}

func defaultBuildTemuInventoryTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
	inventoryService, err := factory.inventoryServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build TEMU inventory service: %w", err)
	}
	return NewInventoryTask(ctx, config, inventoryService), nil
}

func defaultBuildTemuInventoryService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.InventorySyncService, error) {
	temuAPIClient, err := factory.clientManager.GetClient(config.TenantID, config.StoreID)
	if err != nil {
		return nil, fmt.Errorf("get TEMU API client: %w", err)
	}

	inventoryRecordClient := factory.runtime.GetInventoryRecordAPI()

	productFetcher, err := factory.BuildProductFetcher(factory.rabbitmqClient)
	if err != nil {
		return nil, fmt.Errorf("create product fetcher: %w", err)
	}

	inventoryService := schedulerservice.NewInventorySyncService(
		factory.runtime.SyncRuntime(),
		temuAPIClient,
		productFetcher,
		factory.GetMonitorConfig(),
		factory.runtime.GetRawJsonDataAdapter(),
		inventoryRecordClient,
	)

	return newInventorySyncServiceAdapter(inventoryService), nil
}

func defaultBuildTemuActivityTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
	activityService, err := factory.activityServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build TEMU activity service: %w", err)
	}
	return NewActivityTask(ctx, config, activityService), nil
}

func defaultBuildTemuActivityService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (ActivityService, error) {
	_ = config
	_ = factory
	return newNoopActivityService(), nil
}

func (f *TemuTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return f.BaseFactory.SupportedTaskTypes()
}
