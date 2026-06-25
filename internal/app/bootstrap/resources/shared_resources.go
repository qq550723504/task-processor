package resources

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	crawleramazon "task-processor/internal/integration/crawler/amazon"
	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/product"
	"task-processor/internal/prompt"
	"task-processor/internal/taskstatus"
	temupricingruntime "task-processor/internal/temu/pricing"
	temusyncruntime "task-processor/internal/temu/sync"

	"github.com/sirupsen/logrus"
)

// SharedResourceOptions controls which shared runtime dependencies are built.
type SharedResourceOptions struct {
	NeedAmazonCrawler          bool
	AllowMissingManagementAuth bool
	SkipManagementAuth         bool
}

// SharedResources groups dependencies that were previously assembled in multiple places.
type SharedResources struct {
	AuthClient              *auth.ClientCredentialsAuthClient
	ManagementClient        *management.ClientManager
	RawJSONDataClient       product.RawJsonDataClient
	StoreAPI                managementapi.StoreAPI
	SchedulerRuntime        runner.SchedulerRuntimeProvider
	SchedulerFactoryRuntime consumer.SchedulerFactoryRuntime
	ProcessorRuntime        consumer.ProcessorRuntime
	AmazonCrawler           runner.CrawlSource
	RabbitMQClient          *rabbitmq.Client
}

type managementRuntime struct {
	authClient       *auth.ClientCredentialsAuthClient
	managementClient *management.ClientManager
}

// InitializePrompts centralizes prompt registry initialization.
func InitializePrompts(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}

	if err := prompt.InitGlobal(ctx, promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		return fmt.Errorf("initialize prompts: %w", err)
	}

	return nil
}

// BuildSharedResources centralizes auth, management client, and optional crawler assembly.
func BuildSharedResources(cfg *config.Config, logger *logrus.Logger, options SharedResourceOptions) (*SharedResources, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	runtime := &managementRuntime{
		managementClient: newConfiguredManagementClient(cfg, logger),
	}
	if !options.SkipManagementAuth {
		var err error
		runtime, err = buildManagementRuntime(cfg, logger)
		if err != nil {
			if !options.AllowMissingManagementAuth {
				return nil, err
			}

			logger.WithError(err).Warn("management runtime unavailable, continuing without management client")
			runtime = &managementRuntime{
				managementClient: newConfiguredManagementClient(cfg, logger),
			}
		}
	}

	resources := &SharedResources{
		AuthClient:       runtime.authClient,
		ManagementClient: runtime.managementClient,
	}
	if runtime.managementClient != nil {
		resources.RawJSONDataClient = runtime.managementClient.GetRawJsonDataAdapter()
		resources.StoreAPI = runtime.managementClient.GetStoreClient()
		resources.SchedulerRuntime = runtime.managementClient
		resources.SchedulerFactoryRuntime = managementSchedulerFactoryRuntime{source: runtime.managementClient}
		resources.ProcessorRuntime = managementProcessorRuntime{
			managementSchedulerFactoryRuntime: managementSchedulerFactoryRuntime{source: runtime.managementClient},
			source:                            runtime.managementClient,
		}
	}

	if cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled {
		connManager := rabbitmq.NewConnectionManager(rabbitmq.ConnectionConfig{
			URL:               cfg.RabbitMQ.URL,
			ReconnectInterval: cfg.RabbitMQ.ReconnectInterval,
			MaxReconnectTries: cfg.RabbitMQ.MaxReconnectTries,
		}, logger)
		resources.RabbitMQClient = rabbitmq.NewClient(connManager, logger)
	}

	if options.NeedAmazonCrawler {
		resources.AmazonCrawler = BuildAmazonCrawler(cfg, logger)
	}

	return resources, nil
}

func buildManagementRuntime(cfg *config.Config, logger *logrus.Logger) (*managementRuntime, error) {
	tenantID := resolveTenantID(cfg)

	authClient := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		tenantID,
		logger,
	)

	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	managementClient := newConfiguredManagementClient(cfg, logger)
	managementClient.GetClient()
	managementClient.SetUserToken(accessToken, tenantID)

	return &managementRuntime{
		authClient:       authClient,
		managementClient: managementClient,
	}, nil
}

func newConfiguredManagementClient(cfg *config.Config, logger *logrus.Logger) *management.ClientManager {
	managementClient := management.NewClientManager(&cfg.Management)
	managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)

	if provider, err := management.NewLocalDataProvider(cfg.Database, cfg.Redis); err != nil {
		logger.WithError(err).Warn("failed to configure local management data provider")
	} else if provider != nil {
		managementClient.SetLocalDataProvider(provider)
	}

	cookieRedis := cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(cookieRedis.Host) != "" {
		if err := managementClient.SetSheinCookieRedisConfig(&cookieRedis); err != nil {
			logger.WithError(err).Warn("failed to configure SHEIN cookie Redis provider")
		}
	}

	return managementClient
}

// BuildAmazonCrawler constructs the concrete Amazon crawler at the bootstrap edge.
func BuildAmazonCrawler(cfg *config.Config, logger *logrus.Logger) runner.CrawlSource {
	return crawleramazon.NewLegacyCrawlSource(cfg, logger)
}

func resolveTenantID(cfg *config.Config) string {
	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		return "1"
	}

	return tenantID
}

type managementSchedulerFactoryRuntime struct {
	source schedulerRuntimeSource
}

type managementProcessorRuntime struct {
	managementSchedulerFactoryRuntime
	source processorRuntimeSource
}

type schedulerRuntimeSource interface {
	GetRuntimeStoreService() listingruntime.StoreService
	ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error)
	GetStoreClient() *management.StoreAPIClient
	GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error)
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetPricingRuleClient() *management.PricingRuleAPIClient
	GetProductImportMappingClient() *management.ProductImportMappingAPIClient
	GetInventoryRecordClient() *management.InventoryRecordAPIClient
	GetProductDataClient(storeID int64) *management.ProductDataAPIClient
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
}

type processorRuntimeSource interface {
	schedulerRuntimeSource
	GetFilterRuleClient() *management.FilterRuleAPIClient
	GetDailyListingCountClient() *management.DailyListingCountAPIClient
	GetProfitRuleClient() *management.ProfitRuleAPIClient
	GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error)
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
	GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error)
	DeleteSheinStoreCookie(storeID int64) (bool, error)
	GetImageDownloader() *management.ImageDownloader
	SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
	RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error)
	FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error)
	CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error)
	UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error
	GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error)
	GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository
	GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository
}

func (r managementSchedulerFactoryRuntime) GetRuntimeStoreService() listingruntime.StoreService {
	if r.source == nil {
		return nil
	}
	return r.source.GetRuntimeStoreService()
}

func (r managementSchedulerFactoryRuntime) ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.ListRuntimeAutoPricingStoreIDs(ctx, platform)
}

func (r managementSchedulerFactoryRuntime) GetStoreClient() *management.StoreAPIClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreClient()
}

func (r managementSchedulerFactoryRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetAutoPricingStoreConfig(ctx, storeID)
}

func (r managementSchedulerFactoryRuntime) GetRawJsonDataAdapter() product.RawJsonDataClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetRawJsonDataAdapter()
}

func (r managementSchedulerFactoryRuntime) GetStoreAPI() managementapi.StoreAPI {
	return r.GetStoreClient()
}

func (r managementSchedulerFactoryRuntime) GetPricingRuleClient() managementapi.PricingRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetPricingRuleClient()
}

func (r managementSchedulerFactoryRuntime) GetProductImportMappingAPI() managementapi.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingClient()
}

func (r managementSchedulerFactoryRuntime) GetInventoryRecordAPI() managementapi.InventoryRecordAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetInventoryRecordClient()
}

func (r managementSchedulerFactoryRuntime) GetProductDataClient(storeID int64) managementapi.ProductDataAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductDataClient(storeID)
}

func (r managementSchedulerFactoryRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductDataRepository()
}

func (r managementSchedulerFactoryRuntime) PricingRuntime() temupricingruntime.PricingRuntime {
	return temupricingruntime.NewPricingRuntime(r)
}

func (r managementSchedulerFactoryRuntime) SyncRuntime() temusyncruntime.ServiceRuntime {
	if r.source == nil {
		return nil
	}
	return temusyncruntime.NewServiceRuntime(r)
}

func (r managementSchedulerFactoryRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalPricingRuleRepository()
}

func (r managementSchedulerFactoryRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeOperationStrategy(storeID)
}

func (r managementSchedulerFactoryRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductImportMappingRepository()
}

func (r managementSchedulerFactoryRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalStoreRepository()
}

func (r managementSchedulerFactoryRuntime) GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalInventoryRecordRepository()
}

func (r managementSchedulerFactoryRuntime) GetSheinCookie(storeID int64) (string, int64, error) {
	if r.source == nil {
		return "", 0, nil
	}
	return r.source.GetSheinCookie(storeID)
}

func (r managementSchedulerFactoryRuntime) GetSheinStoreCookie(storeID int64) (string, error) {
	if r.source == nil {
		return "", nil
	}
	return r.source.GetSheinStoreCookie(storeID)
}

func (r managementProcessorRuntime) GetFilterRuleClient() *management.FilterRuleAPIClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetFilterRuleClient()
}

func (r managementProcessorRuntime) GetDailyListingCountClient() *management.DailyListingCountAPIClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetDailyListingCountClient()
}

func (r managementProcessorRuntime) GetProductImportMappingClient() *management.ProductImportMappingAPIClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingClient()
}

func (r managementProcessorRuntime) GetProfitRuleClient() *management.ProfitRuleAPIClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetProfitRuleClient()
}

func (r managementProcessorRuntime) GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalFilterRuleRepository()
}

func (r managementProcessorRuntime) GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProfitRuleRepository()
}

func (r managementProcessorRuntime) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetTaskStatus(taskID)
}

func (r managementProcessorRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r.source == nil {
		return nil
	}
	return r.source.UpdateRuntimeTaskStatus(req)
}

func (r managementProcessorRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeImportTask(taskID)
}

func (r managementProcessorRuntime) DeleteSheinStoreCookie(storeID int64) (bool, error) {
	if r.source == nil {
		return false, nil
	}
	return r.source.DeleteSheinStoreCookie(storeID)
}

func (r managementProcessorRuntime) GetImageDownloader() *management.ImageDownloader {
	if r.source == nil {
		return nil
	}
	return r.source.GetImageDownloader()
}

func (r managementProcessorRuntime) SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	if r.source == nil {
		return false, nil
	}
	return r.source.SetRuntimeStorePauseStatus(storeID, pause, pauseType)
}

func (r managementProcessorRuntime) RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error) {
	if r.source == nil {
		return false, nil
	}
	return r.source.RuntimePublishedProductExists(ctx, storeID, platform, region, productID)
}

func (r managementProcessorRuntime) FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.FindRuntimeProductImportMappingByTaskAndSKU(ctx, importTaskID, sku)
}

func (r managementProcessorRuntime) CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	if r.source == nil {
		return 0, nil
	}
	return r.source.CreateRuntimeProductImportMapping(ctx, req)
}

func (r managementProcessorRuntime) UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error {
	if r.source == nil {
		return nil
	}
	return r.source.UpdateRuntimeProductImportMapping(ctx, req)
}

func (r managementProcessorRuntime) GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeStorePauseStatusDetail(storeID)
}
