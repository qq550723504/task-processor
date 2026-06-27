package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/platformbase"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/ports"
	"task-processor/internal/pricing"
	appscheduler "task-processor/internal/scheduler"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
	sheinpricingapi "task-processor/internal/shein/api/pricing"
	sheinproductapi "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/client"
	"task-processor/internal/shein/inventory"
	sheinmanagedclient "task-processor/internal/shein/managedclient"
	sheinpricing "task-processor/internal/shein/pricing"
	"task-processor/internal/shein/productsync"
	"task-processor/internal/state"
)

type schedulerRuntime interface {
	platformbase.Runtime
	platformtask.AutoPricingStoreConfigProvider
	pricing.OperationStrategyProvider
	GetRuntimeStoreService() listingruntime.StoreService
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
}

type TaskBuilder func(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error)
type PricingServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.AutoPricingService, error)
type ProductSyncServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.ProductSyncService, error)
type InventoryServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.InventorySyncService, error)
type ActivityServiceBuilder func(config appscheduler.TaskConfig, factory *SheinTaskFactory) (activity.ActivityRegistrationService, error)

type Dependencies struct {
	CookieManager             *state.CookieManager
	ClientManager             *sheinmanagedclient.ClientManager
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
	clientManager             *sheinmanagedclient.ClientManager
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
	runtimeProvider schedulerRuntime,
	crawlSource ports.CrawlSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) *SheinTaskFactory {
	cookieManager := state.NewCookieManager()
	storeService := runtimeProvider.GetRuntimeStoreService()
	return NewSheinTaskFactoryWithDependencies(
		runtimeProvider,
		crawlSource,
		amazonConfig,
		monitorConfig,
		rabbitmqClient,
		Dependencies{
			CookieManager:  cookieManager,
			ClientManager:  sheinmanagedclient.NewClientManager(cookieManager, sheinmanagedclient.NewRuntimeCookieProvider(runtimeProvider, storeService), sheinmanagedclient.NewRuntimeStoreConfigProvider(storeService)),
			FetcherBuilder: platformbase.NewDefaultProductFetcherBuilder(),
		},
	)
}

func NewSheinTaskFactoryWithFetcherBuilder(
	runtimeProvider schedulerRuntime,
	fetcherBuilder platformbase.ProductFetcherBuilder,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) *SheinTaskFactory {
	cookieManager := state.NewCookieManager()
	storeService := runtimeProvider.GetRuntimeStoreService()
	return NewSheinTaskFactoryWithDependencies(
		runtimeProvider,
		nil,
		amazonConfig,
		monitorConfig,
		rabbitmqClient,
		Dependencies{
			CookieManager:  cookieManager,
			ClientManager:  sheinmanagedclient.NewClientManager(cookieManager, sheinmanagedclient.NewRuntimeCookieProvider(runtimeProvider, storeService), sheinmanagedclient.NewRuntimeStoreConfigProvider(storeService)),
			FetcherBuilder: fetcherBuilder,
		},
	)
}

func NewSheinTaskFactoryWithDependencies(
	runtimeProvider schedulerRuntime,
	crawlSource ports.CrawlSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
	deps Dependencies,
) *SheinTaskFactory {
	_ = crawlSource
	baseFactory := platformbase.NewBaseFactory(platformbase.BaseFactoryConfig{
		Platform:       "SHEIN",
		Runtime:        runtimeProvider,
		FetcherBuilder: deps.FetcherBuilder,
		AmazonConfig:   amazonConfig,
		MonitorConfig:  monitorConfig,
	})

	factory := &SheinTaskFactory{
		BaseFactory:    baseFactory,
		cookieManager:  deps.CookieManager,
		clientManager:  deps.ClientManager,
		rabbitmqClient: rabbitmqClient,
		fetcherBuilder: baseFactory.GetFetcherBuilder(),
	}

	if factory.cookieManager == nil {
		factory.cookieManager = state.NewCookieManager()
	}
	if factory.clientManager == nil {
		storeService := factory.runtimeProvider().GetRuntimeStoreService()
		factory.clientManager = sheinmanagedclient.NewClientManager(
			factory.cookieManager,
			sheinmanagedclient.NewRuntimeCookieProvider(factory.runtimeProvider(), storeService),
			sheinmanagedclient.NewRuntimeStoreConfigProvider(storeService),
		)
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
	return NewPricingTask(ctx, config, factory.runtimeProvider(), pricingService), nil
}

func defaultBuildSheinPricingService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.AutoPricingService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	pricingAPI := sheinpricingapi.NewClient(baseClient)
	pricingService := sheinpricing.NewAutoPricingService(
		factory.runtimeProvider().GetLocalPricingRuleRepository(),
		factory.runtimeProvider().GetLocalProductImportMappingRepository(),
		pricingAPI,
	)
	return NewSheinAutoPricingAdapter(pricingService), nil
}

func defaultBuildSheinProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	syncService, err := factory.productSyncServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN product sync service: %w", err)
	}
	return NewProductSyncTask(ctx, config, syncService), nil
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

	syncService := productsync.NewProductSyncService(
		productAPI,
		inventoryManager,
		priceManager,
		factory.runtimeProvider().GetRuntimeStoreService(),
		factory.runtimeProvider().GetLocalStoreRepository(),
		factory.runtimeProvider().GetLocalProductImportMappingRepository(),
		factory.runtimeProvider().GetLocalProductDataRepository(),
	)

	return newProductSyncServiceAdapter(syncService), nil
}

func defaultBuildSheinInventoryTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	inventoryService, err := factory.inventoryServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN inventory service: %w", err)
	}
	return NewInventoryTask(ctx, config, inventoryService), nil
}

func defaultBuildSheinInventoryService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (platformtask.InventorySyncService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	productAPI := sheinproductapi.NewClient(baseClient)
	inventoryRecordRepo := factory.runtimeProvider().GetLocalInventoryRecordRepository()

	productFetcher, err := factory.BuildProductFetcher(factory.rabbitmqClient)
	if err != nil {
		return nil, fmt.Errorf("create product fetcher: %w", err)
	}

	inventoryService := inventory.NewInventorySyncService(
		factory.runtimeProvider(),
		productAPI,
		productFetcher,
		factory.GetMonitorConfig(),
		factory.runtimeProvider().GetRawJsonDataAdapter(),
		inventoryRecordRepo,
		factory.runtimeProvider().GetRuntimeStoreService(),
		factory.runtimeProvider().GetLocalStoreRepository(),
		factory.runtimeProvider().GetLocalProductDataRepository(),
	)

	return newInventorySyncServiceAdapter(inventoryService), nil
}

func defaultBuildSheinActivityTask(ctx context.Context, config appscheduler.TaskConfig, factory *SheinTaskFactory) (appscheduler.Task, error) {
	activityService, err := factory.activityServiceBuilder(config, factory)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN activity service: %w", err)
	}
	return NewActivityTask(ctx, config, factory.runtimeProvider(), activityService), nil
}

func defaultBuildSheinActivityService(config appscheduler.TaskConfig, factory *SheinTaskFactory) (activity.ActivityRegistrationService, error) {
	baseClient, err := factory.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	marketingAPI := marketing.NewClient(baseClient)
	return activity.NewActivityRegistrationService(
		factory.runtimeProvider().GetRuntimeStoreService(),
		factory.runtimeProvider().GetLocalStoreRepository(),
		factory.runtimeProvider().GetLocalProductImportMappingRepository(),
		factory.runtimeProvider().GetLocalProductDataRepository(),
		marketingAPI,
	), nil
}

func (f *SheinTaskFactory) createBaseClient(storeID int64) (*client.BaseAPIClient, error) {
	runtimeStoreService := f.runtimeProvider().GetRuntimeStoreService()
	if runtimeStoreService == nil {
		return nil, fmt.Errorf("runtime store service is not initialized")
	}

	storeInfo, err := runtimeStoreService.GetStore(storeID)
	if err != nil {
		return nil, fmt.Errorf("get runtime store info: %w", err)
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

func (f *SheinTaskFactory) runtimeProvider() schedulerRuntime {
	if f == nil || f.BaseFactory == nil {
		return nil
	}
	runtimeProvider, _ := f.GetRuntime().(schedulerRuntime)
	return runtimeProvider
}
