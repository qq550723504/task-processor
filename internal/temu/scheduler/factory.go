package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/app/ports"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	platformtask "task-processor/internal/platformtask"
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
	managementClient *management.ClientManager,
	amazonProcessor ports.ProductSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) *TemuTaskFactory {
	return NewTemuTaskFactoryWithDependencies(
		managementClient,
		amazonProcessor,
		amazonConfig,
		monitorConfig,
		rabbitmqClient,
		Dependencies{
			ClientManager:  client.NewAPIClientManager(managementClient),
			FetcherBuilder: platformbase.NewDefaultProductFetcherBuilder(),
		},
	)
}

func NewTemuTaskFactoryWithDependencies(
	managementClient *management.ClientManager,
	amazonProcessor ports.ProductSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
	deps Dependencies,
) *TemuTaskFactory {
	baseFactory := platformbase.NewBaseFactory(platformbase.BaseFactoryConfig{
		Platform:         "TEMU",
		ManagementClient: managementClient,
		AmazonProcessor:  amazonProcessor,
		AmazonConfig:     amazonConfig,
		MonitorConfig:    monitorConfig,
	})

	factory := &TemuTaskFactory{
		BaseFactory:    baseFactory,
		clientManager:  deps.ClientManager,
		rabbitmqClient: rabbitmqClient,
		fetcherBuilder: deps.FetcherBuilder,
	}

	if factory.clientManager == nil {
		factory.clientManager = client.NewAPIClientManager(managementClient)
	}
	if factory.fetcherBuilder == nil {
		factory.fetcherBuilder = platformbase.NewDefaultProductFetcherBuilder()
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
	return NewPricingTask(ctx, config, factory.GetManagementClient(), pricingService), nil
}

func defaultBuildTemuPricingService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.AutoPricingService, error) {
	apiClient := temuapi.NewAPIClient(config.StoreID, factory.GetManagementClient())
	if apiClient == nil {
		return nil, fmt.Errorf("create TEMU API client")
	}
	return NewTemuAutoPricingAdapter(apiClient, factory.GetManagementClient()), nil
}

func defaultBuildTemuProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
	syncService, err := factory.productSyncServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build TEMU product sync service: %w", err)
	}
	return NewProductSyncTask(ctx, config, factory.GetManagementClient(), syncService), nil
}

func defaultBuildTemuProductSyncService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.ProductSyncService, error) {
	apiClient, err := factory.clientManager.GetClient(config.TenantID, config.StoreID)
	if err != nil {
		return nil, fmt.Errorf("get TEMU API client: %w", err)
	}

	productAPI := temuapi.NewProductAPI(apiClient, logger.GetGlobalLogger("TemuProductAPI"))
	skuQueryAPI := temuapi.NewQueryAPI(apiClient, logger.GetGlobalLogger("TemuSkuQueryAPI"))
	mappingClient := factory.GetManagementClient().GetProductImportMappingClient()
	storeAPI := factory.GetManagementClient().GetStoreClient()

	syncConfig := &schedulerservice.ProductSyncConfig{
		PageSize:        100,
		MaxPages:        0,
		Language:        "en",
		IncludeInactive: false,
	}

	syncService := schedulerservice.NewProductSyncService(
		factory.GetManagementClient(),
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
	return NewInventoryTask(ctx, config, factory.GetManagementClient(), inventoryService), nil
}

func defaultBuildTemuInventoryService(config appscheduler.TaskConfig, factory *TemuTaskFactory) (platformtask.InventorySyncService, error) {
	temuAPIClient, err := factory.clientManager.GetClient(config.TenantID, config.StoreID)
	if err != nil {
		return nil, fmt.Errorf("get TEMU API client: %w", err)
	}

	rawJSONDataClient := factory.GetManagementClient().GetRawJsonDataAdapter()
	inventoryRecordClient := factory.GetManagementClient().GetInventoryRecordClient()

	productFetcher, err := factory.fetcherBuilder.Build(
		rawJSONDataClient,
		factory.GetAmazonConfig(),
		factory.GetAmazonProcessor(),
		factory.rabbitmqClient,
	)
	if err != nil {
		return nil, fmt.Errorf("create product fetcher: %w", err)
	}

	inventoryService := schedulerservice.NewInventorySyncService(
		factory.GetManagementClient(),
		temuAPIClient,
		productFetcher,
		factory.GetMonitorConfig(),
		rawJSONDataClient,
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
