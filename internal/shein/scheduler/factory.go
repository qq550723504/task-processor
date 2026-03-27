package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/app/ports"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/app/state"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
	sheinpricingapi "task-processor/internal/shein/api/pricing"
	sheinproductapi "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/client"
	"task-processor/internal/shein/inventory"
	sheinpricing "task-processor/internal/shein/pricing"
	"task-processor/internal/shein/productsync"
)

type TaskBuilder func(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error)
type PricingServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.AutoPricingService, error)
type ProductSyncServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.ProductSyncService, error)
type InventoryServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.InventorySyncService, error)
type ActivityServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (activity.ActivityRegistrationService, error)

type Dependencies struct {
	CookieManager             *state.CookieManager
	ClientManager             *client.ClientManager
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

type SheinTaskFactory struct {
	*platformbase.BaseFactory
	cookieManager             *state.CookieManager
	clientManager             *client.ClientManager
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

func NewSheinTaskFactory(
	managementClient *management.ClientManager,
	amazonProcessor ports.ProductSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) *SheinTaskFactory {
	cookieManager := state.NewCookieManager()
	return NewSheinTaskFactoryWithDependencies(
		managementClient,
		amazonProcessor,
		amazonConfig,
		monitorConfig,
		rabbitmqClient,
		Dependencies{
			CookieManager:  cookieManager,
			ClientManager:  client.NewClientManager(cookieManager, managementClient),
			FetcherBuilder: platformbase.NewDefaultProductFetcherBuilder(),
		},
	)
}

func NewSheinTaskFactoryWithDependencies(
	managementClient *management.ClientManager,
	amazonProcessor ports.ProductSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
	deps Dependencies,
) *SheinTaskFactory {
	baseFactory := platformbase.NewBaseFactory(platformbase.BaseFactoryConfig{
		Platform:         "SHEIN",
		ManagementClient: managementClient,
		AmazonProcessor:  amazonProcessor,
		AmazonConfig:     amazonConfig,
		MonitorConfig:    monitorConfig,
	})

	factory := &SheinTaskFactory{
		BaseFactory:    baseFactory,
		cookieManager:  deps.CookieManager,
		clientManager:  deps.ClientManager,
		rabbitmqClient: rabbitmqClient,
		fetcherBuilder: deps.FetcherBuilder,
	}

	if factory.cookieManager == nil {
		factory.cookieManager = state.NewCookieManager()
	}
	if factory.clientManager == nil {
		factory.clientManager = client.NewClientManager(factory.cookieManager, managementClient)
	}
	if factory.fetcherBuilder == nil {
		factory.fetcherBuilder = platformbase.NewDefaultProductFetcherBuilder()
	}
	factory.pricingServiceBuilder = deps.PricingServiceBuilder
	if factory.pricingServiceBuilder == nil {
		factory.pricingServiceBuilder = defaultBuildSheinPricingService
	}
	factory.productSyncServiceBuilder = deps.ProductSyncServiceBuilder
	if factory.productSyncServiceBuilder == nil {
		factory.productSyncServiceBuilder = defaultBuildSheinProductSyncService
	}
	factory.inventoryServiceBuilder = deps.InventoryServiceBuilder
	if factory.inventoryServiceBuilder == nil {
		factory.inventoryServiceBuilder = defaultBuildSheinInventoryService
	}
	factory.activityServiceBuilder = deps.ActivityServiceBuilder
	if factory.activityServiceBuilder == nil {
		factory.activityServiceBuilder = defaultBuildSheinActivityService
	}
	factory.pricingTaskBuilder = deps.PricingTaskBuilder
	if factory.pricingTaskBuilder == nil {
		factory.pricingTaskBuilder = defaultBuildSheinPricingTask
	}
	factory.productSyncTaskBuilder = deps.ProductSyncTaskBuilder
	if factory.productSyncTaskBuilder == nil {
		factory.productSyncTaskBuilder = defaultBuildSheinProductSyncTask
	}
	factory.inventoryTaskBuilder = deps.InventoryTaskBuilder
	if factory.inventoryTaskBuilder == nil {
		factory.inventoryTaskBuilder = defaultBuildSheinInventoryTask
	}
	factory.activityTaskBuilder = deps.ActivityTaskBuilder
	if factory.activityTaskBuilder == nil {
		factory.activityTaskBuilder = defaultBuildSheinActivityTask
	}

	return factory
}

func (f *SheinTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
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

func defaultBuildSheinPricingTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	pricingService, err := factory.pricingServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN pricing service: %w", err)
	}
	return NewPricingTask(ctx, config, factory.GetManagementClient(), pricingService), nil
}

func defaultBuildSheinPricingService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.AutoPricingService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	pricingAPI := sheinpricingapi.NewClient(baseClient)
	pricingService := sheinpricing.NewAutoPricingService(factory.GetManagementClient(), pricingAPI)
	return NewSheinAutoPricingAdapter(pricingService), nil
}

func defaultBuildSheinProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	syncService, err := factory.productSyncServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN product sync service: %w", err)
	}
	return NewProductSyncTask(ctx, config, factory.GetManagementClient(), syncService), nil
}

func defaultBuildSheinProductSyncService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.ProductSyncService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	errorHandler := client.NewAPIErrorHandler(baseClient)
	productAPI := sheinproductapi.NewClient(baseClient)
	inventoryManager := sheinproductapi.NewInventoryManager(baseClient, errorHandler)
	priceManager := sheinproductapi.NewPriceManager(baseClient, errorHandler)
	storeInfoClient := factory.GetManagementClient().GetStoreClient()
	mappingClient := factory.GetManagementClient().GetProductImportMappingClient()

	syncService := productsync.NewProductSyncService(
		factory.GetManagementClient(),
		productAPI,
		inventoryManager,
		priceManager,
		mappingClient,
		storeInfoClient,
	)

	return newProductSyncServiceAdapter(syncService), nil
}

func defaultBuildSheinInventoryTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	inventoryService, err := factory.inventoryServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN inventory service: %w", err)
	}
	return NewInventoryTask(ctx, config, factory.GetManagementClient(), inventoryService), nil
}

func defaultBuildSheinInventoryService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.InventorySyncService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	productAPI := sheinproductapi.NewClient(baseClient)
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

	inventoryService := inventory.NewInventorySyncService(
		factory.GetManagementClient(),
		productAPI,
		productFetcher,
		factory.GetMonitorConfig(),
		rawJSONDataClient,
		inventoryRecordClient,
	)

	return newInventorySyncServiceAdapter(inventoryService), nil
}

func defaultBuildSheinActivityTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	activityService, err := factory.activityServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN activity service: %w", err)
	}
	return NewActivityTask(ctx, config, factory.GetManagementClient(), activityService), nil
}

func defaultBuildSheinActivityService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (activity.ActivityRegistrationService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	marketingAPI := marketing.NewClient(baseClient)
	return activity.NewActivityRegistrationService(factory.GetManagementClient(), marketingAPI), nil
}

func (f *SheinTaskFactory) createBaseClient(storeID int64) (*client.BaseAPIClient, error) {
	storeInfo, err := f.GetManagementClient().GetStoreClient().GetStore(storeID)
	if err != nil {
		return nil, fmt.Errorf("get store info: %w", err)
	}

	apiClient, err := f.clientManager.GetClient(storeID, storeInfo)
	if err != nil {
		return nil, fmt.Errorf("get API client: %w", err)
	}

	return client.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		apiClient.GetStoreID(),
		apiClient.GetHTTPClient(),
	), nil
}

func (f *SheinTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return f.BaseFactory.SupportedTaskTypes()
}
