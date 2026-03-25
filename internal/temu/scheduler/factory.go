package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	temuapi "task-processor/internal/temu/api"
	"task-processor/internal/temu/api/client"
	schedulerservice "task-processor/internal/temu/sync"
)

type TaskBuilder func(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error)

type Dependencies struct {
	ClientManager          *client.APIClientManager
	FetcherBuilder         platformbase.ProductFetcherBuilder
	PricingTaskBuilder     TaskBuilder
	ProductSyncTaskBuilder TaskBuilder
	InventoryTaskBuilder   TaskBuilder
	ActivityTaskBuilder    TaskBuilder
}

type TemuTaskFactory struct {
	*platformbase.BaseFactory
	clientManager          *client.APIClientManager
	rabbitmqClient         *rabbitmq.Client
	fetcherBuilder         platformbase.ProductFetcherBuilder
	pricingTaskBuilder     TaskBuilder
	productSyncTaskBuilder TaskBuilder
	inventoryTaskBuilder   TaskBuilder
	activityTaskBuilder    TaskBuilder
}

func NewTemuTaskFactory(
	managementClient *management.ClientManager,
	amazonProcessor platformbase.AmazonCrawler,
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
	amazonProcessor platformbase.AmazonCrawler,
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
	return NewPricingTask(ctx, config, factory.GetManagementClient()), nil
}

func defaultBuildTemuProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
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

	return NewProductSyncTask(ctx, config, factory.GetManagementClient(), syncService), nil
}

func defaultBuildTemuInventoryTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
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

	return NewInventoryTask(ctx, config, factory.GetManagementClient(), inventoryService), nil
}

func defaultBuildTemuActivityTask(ctx context.Context, config appscheduler.TaskConfig, factory *TemuTaskFactory) (appscheduler.Task, error) {
	return NewActivityTask(ctx, config, factory.GetManagementClient()), nil
}

func (f *TemuTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return f.BaseFactory.SupportedTaskTypes()
}
