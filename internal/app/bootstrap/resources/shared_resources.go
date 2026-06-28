package resources

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/consumer"
	"task-processor/internal/app/ports"
	"task-processor/internal/app/runner"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/rabbitmq"
	crawleramazon "task-processor/internal/integration/crawler/amazon"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	localruntime "task-processor/internal/listingruntime/local"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/product"
	"task-processor/internal/prompt"
	temupricingruntime "task-processor/internal/temu/pricing"
	temusyncruntime "task-processor/internal/temu/sync"

	"github.com/sirupsen/logrus"
)

// SharedResourceOptions controls which shared runtime dependencies are built.
type SharedResourceOptions struct {
	NeedAmazonCrawler bool
}

// SharedResources groups dependencies that were previously assembled in multiple places.
type SharedResources struct {
	AuthClient                    *auth.ClientCredentialsAuthClient
	ListingRuntimeHealthValidator interface {
		ValidateLocalListingRuntimeFields() (map[string]bool, error)
	}
	RawJSONDataClient       product.RawJsonDataClient
	StoreAPI                listingadmin.StoreAPI
	SchedulerRuntime        runner.SchedulerRuntimeProvider
	SchedulerFactoryRuntime consumer.SchedulerFactoryRuntime
	ProcessorRuntime        consumer.ProcessorRuntime
	ImportTaskRepository    consumer.ListingRuntimeImportTaskRepository
	AmazonCrawler           ports.CrawlSource
	RabbitMQClient          *rabbitmq.Client
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

// BuildSharedResources centralizes local listing runtime and optional crawler assembly.
func BuildSharedResources(cfg *config.Config, logger *logrus.Logger, options SharedResourceOptions) (*SharedResources, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	localProvider, err := localruntime.NewDataProvider(cfg.Database, cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("configure local listing runtime data provider: %w", err)
	}

	var cookieProvider localruntime.SheinCookieProvider
	cookieRedis := cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(cookieRedis.Host) != "" {
		if provider, err := localruntime.NewRedisSheinCookieProvider(&cookieRedis); err != nil {
			logger.WithError(err).Warn("failed to configure SHEIN cookie Redis provider")
		} else {
			cookieProvider = provider
		}
	}

	localRuntime := localruntime.NewRuntime(localProvider, localruntime.RuntimeOptions{
		SheinCookieProvider: cookieProvider,
	})

	resources := &SharedResources{}
	if localRuntime != nil {
		resources.ListingRuntimeHealthValidator = localRuntime
		resources.RawJSONDataClient = localruntime.NewRawJSONDataAdapter(localProvider)
		resources.StoreAPI = localRuntime.GetStoreAPI()
		resources.SchedulerRuntime = localRuntime
		resources.SchedulerFactoryRuntime = localSchedulerFactoryRuntime{source: localRuntime}
		resources.ProcessorRuntime = localProcessorRuntime{
			localSchedulerFactoryRuntime: localSchedulerFactoryRuntime{source: localRuntime},
			source:                       localRuntime,
		}
		resources.ImportTaskRepository = localProvider.ImportTaskRepository()
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

// BuildAmazonCrawler constructs the concrete Amazon crawler at the bootstrap edge.
func BuildAmazonCrawler(cfg *config.Config, logger *logrus.Logger) ports.CrawlSource {
	return crawleramazon.NewLegacyCrawlSource(cfg, logger)
}

type localSchedulerFactoryRuntime struct {
	source schedulerRuntimeSource
}

type localProcessorRuntime struct {
	localSchedulerFactoryRuntime
	source processorRuntimeSource
}

type schedulerRuntimeSource interface {
	GetRuntimeStoreService() listingruntime.StoreService
	ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error)
	GetStoreAPI() listingadmin.StoreAPI
	GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error)
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetPricingRuleClient() listingadmin.PricingRuleAPI
	GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI
	GetInventoryRecordAPI() listingadmin.InventoryRecordAPI
	GetProductDataClient(storeID int64) listingadmin.ProductDataAPI
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
	GetFilterRuleClient() listingadmin.FilterRuleAPI
	GetDailyListingCountClient() listingadmin.DailyListingCountAPI
	GetProfitRuleClient() listingadmin.ProfitRuleAPI
	GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error)
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
	GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error)
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
	GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository
	GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository
}

func (r localSchedulerFactoryRuntime) GetRuntimeStoreService() listingruntime.StoreService {
	if r.source == nil {
		return nil
	}
	return r.source.GetRuntimeStoreService()
}

func (r localSchedulerFactoryRuntime) ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.ListRuntimeAutoPricingStoreIDs(ctx, platform)
}

func (r localSchedulerFactoryRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetAutoPricingStoreConfig(ctx, storeID)
}

func (r localSchedulerFactoryRuntime) GetRawJsonDataAdapter() product.RawJsonDataClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetRawJsonDataAdapter()
}

func (r localSchedulerFactoryRuntime) GetStoreAPI() listingadmin.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreAPI()
}

func (r localSchedulerFactoryRuntime) GetPricingRuleClient() listingadmin.PricingRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetPricingRuleClient()
}

func (r localSchedulerFactoryRuntime) GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingAPI()
}

func (r localSchedulerFactoryRuntime) GetInventoryRecordAPI() listingadmin.InventoryRecordAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetInventoryRecordAPI()
}

func (r localSchedulerFactoryRuntime) GetProductDataClient(storeID int64) listingadmin.ProductDataAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductDataClient(storeID)
}

func (r localSchedulerFactoryRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductDataRepository()
}

func (r localSchedulerFactoryRuntime) PricingRuntime() temupricingruntime.PricingRuntime {
	return temupricingruntime.NewPricingRuntime(r)
}

func (r localSchedulerFactoryRuntime) SyncRuntime() temusyncruntime.ServiceRuntime {
	if r.source == nil {
		return nil
	}
	return temusyncruntime.NewServiceRuntime(r)
}

func (r localSchedulerFactoryRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalPricingRuleRepository()
}

func (r localSchedulerFactoryRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeOperationStrategy(storeID)
}

func (r localSchedulerFactoryRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductImportMappingRepository()
}

func (r localSchedulerFactoryRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalStoreRepository()
}

func (r localSchedulerFactoryRuntime) GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalInventoryRecordRepository()
}

func (r localSchedulerFactoryRuntime) GetSheinCookie(storeID int64) (string, int64, error) {
	if r.source == nil {
		return "", 0, nil
	}
	return r.source.GetSheinCookie(storeID)
}

func (r localSchedulerFactoryRuntime) GetSheinStoreCookie(storeID int64) (string, error) {
	if r.source == nil {
		return "", nil
	}
	return r.source.GetSheinStoreCookie(storeID)
}

func (r localProcessorRuntime) GetFilterRuleClient() listingadmin.FilterRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetFilterRuleClient()
}

func (r localProcessorRuntime) GetDailyListingCountClient() listingadmin.DailyListingCountAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetDailyListingCountClient()
}

func (r localProcessorRuntime) GetProductImportMappingClient() listingadmin.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingAPI()
}

func (r localProcessorRuntime) GetProfitRuleClient() listingadmin.ProfitRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProfitRuleClient()
}

func (r localProcessorRuntime) GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalFilterRuleRepository()
}

func (r localProcessorRuntime) GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProfitRuleRepository()
}

func (r localProcessorRuntime) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetTaskStatus(taskID)
}

func (r localProcessorRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r.source == nil {
		return nil
	}
	return r.source.UpdateRuntimeTaskStatus(req)
}

func (r localProcessorRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeImportTask(taskID)
}

func (r localProcessorRuntime) DeleteSheinStoreCookie(storeID int64) (bool, error) {
	if r.source == nil {
		return false, nil
	}
	return r.source.DeleteSheinStoreCookie(storeID)
}

func (r localProcessorRuntime) GetImageDownloader() interface {
	DownloadImage(url string) ([]byte, error)
} {
	if r.source == nil {
		return nil
	}
	return r.source.GetImageDownloader()
}

func (r localProcessorRuntime) SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	if r.source == nil {
		return false, nil
	}
	return r.source.SetRuntimeStorePauseStatus(storeID, pause, pauseType)
}

func (r localProcessorRuntime) RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error) {
	if r.source == nil {
		return false, nil
	}
	return r.source.RuntimePublishedProductExists(ctx, storeID, platform, region, productID)
}

func (r localProcessorRuntime) FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.FindRuntimeProductImportMappingByTaskAndSKU(ctx, importTaskID, sku)
}

func (r localProcessorRuntime) CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	if r.source == nil {
		return 0, nil
	}
	return r.source.CreateRuntimeProductImportMapping(ctx, req)
}

func (r localProcessorRuntime) UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error {
	if r.source == nil {
		return nil
	}
	return r.source.UpdateRuntimeProductImportMapping(ctx, req)
}

func (r localProcessorRuntime) GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeStorePauseStatusDetail(storeID)
}
