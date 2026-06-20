package pipeline

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	types "task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/processor"
	"task-processor/internal/shein/aicache"
	sheinclient "task-processor/internal/shein/client"
	sheincontext "task-processor/internal/shein/context"
	sheinmanagedclient "task-processor/internal/shein/managedclient"
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"

	"github.com/sirupsen/logrus"
)

type managementRuntime interface {
	sheincontext.RuntimeRepository
	GetRuntimeStoreService() listingruntime.StoreService
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository
	GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
	DeleteSheinStoreCookie(storeID int64) (bool, error)
	SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
}

type Dependencies struct {
	ManagementClient  managementRuntime
	TaskStatusRuntime taskstatus.RuntimeWithTaskRPC
	MemoryManager     *state.MemoryManager
	ImageDownloader   interface {
		DownloadImage(url string) ([]byte, error)
	}
	ProductFetcher appfetcher.ProductFetcher
	RabbitMQClient *rabbitmq.Client
}

type SheinProcessor struct {
	*processor.BaseProcessor
	managementClient  managementRuntime
	taskStatusRuntime taskstatus.RuntimeWithTaskRPC
	imageDownloader   interface {
		DownloadImage(url string) ([]byte, error)
	}
	productFetcher appfetcher.ProductFetcher
	rabbitmqClient *rabbitmq.Client
	taskHandler    *TaskHandler
	pipeline       *Pipeline
	aiCache        *aicache.Cache
}

func NewSheinProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps Dependencies) (*SheinProcessor, error) {
	if deps.ManagementClient == nil {
		logger.Error("[SHEIN] ManagementClient is required")
		return nil, fmt.Errorf("managementClient is required")
	}
	if deps.ProductFetcher == nil {
		logger.Error("[SHEIN] ProductFetcher is required")
		return nil, fmt.Errorf("productFetcher is required")
	}
	if deps.TaskStatusRuntime == nil {
		logger.Error("[SHEIN] TaskStatusRuntime is required")
		return nil, fmt.Errorf("taskStatusRuntime is required")
	}
	if deps.MemoryManager == nil {
		logger.Error("[SHEIN] MemoryManager is required")
		return nil, fmt.Errorf("memoryManager is required")
	}
	if deps.ImageDownloader == nil {
		logger.Error("[SHEIN] ImageDownloader is required")
		return nil, fmt.Errorf("imageDownloader is required")
	}

	if deps.RabbitMQClient != nil {
		logger.Info("[SHEIN] using RabbitMQ client for distributed fetching")
	} else {
		logger.Warn("[SHEIN] RabbitMQ client not provided; distributed fetching is unavailable")
	}

	baseProcessor := processor.NewBaseProcessorWithMemoryManager(&processor.BaseProcessorConfig{
		Config:   cfg,
		Logger:   logger,
		Platform: "SHEIN",
	}, deps.MemoryManager)

	p := &SheinProcessor{
		BaseProcessor:     baseProcessor,
		managementClient:  deps.ManagementClient,
		taskStatusRuntime: deps.TaskStatusRuntime,
		imageDownloader:   deps.ImageDownloader,
		productFetcher:    deps.ProductFetcher,
		rabbitmqClient:    deps.RabbitMQClient,
	}

	if cfg.Database == nil {
		logger.Warn("[SHEIN] database config is nil, AI cache will fall back to memory")
	}
	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		logger.Warnf("[SHEIN] database unavailable, AI cache falling back to memory: %v", err)
	}
	p.aiCache = aicache.New(db)

	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)
	p.taskHandler = NewTaskHandler(p)
	p.pipeline = p.buildPipeline()

	return p, nil
}

func (p *SheinProcessor) buildPipeline() *Pipeline {
	return CreateTaskProcessingPipeline(p, p.GetConfig())
}

func (p *SheinProcessor) Start(ctx context.Context) error {
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	p.GetLogger().Info("[SHEIN] processor started")
	return nil
}

func (p *SheinProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var task types.Task
	if err := jsonx.UnmarshalString(job.TaskData, &task, "parse task data"); err != nil {
		return err
	}

	return p.taskHandler.ProcessTask(ctx, task, p.pipeline)
}

func (p *SheinProcessor) GetAICache() *aicache.Cache {
	return p.aiCache
}

func (p *SheinProcessor) GetProductFetcher() appfetcher.ProductFetcher {
	return p.productFetcher
}

func (p *SheinProcessor) GetRuntimeRepository() sheincontext.RuntimeRepository {
	if p == nil {
		return nil
	}
	return p.managementClient
}

func (p *SheinProcessor) GetTaskStatusRuntime() taskstatus.RuntimeWithTaskRPC {
	if p == nil {
		return nil
	}
	return p.taskStatusRuntime
}

func (p *SheinProcessor) GetRuntimeStoreService() listingruntime.StoreService {
	if p == nil || p.managementClient == nil {
		return nil
	}
	return p.managementClient.GetRuntimeStoreService()
}

func (p *SheinProcessor) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if p == nil || p.managementClient == nil {
		return nil
	}
	return p.managementClient.GetLocalStoreRepository()
}

func (p *SheinProcessor) GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	if p == nil || p.managementClient == nil {
		return nil
	}
	return p.managementClient.GetLocalFilterRuleRepository()
}

func (p *SheinProcessor) GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	if p == nil || p.managementClient == nil {
		return nil
	}
	return p.managementClient.GetLocalProfitRuleRepository()
}

func (p *SheinProcessor) GetSheinCookie(storeID int64) (string, int64, error) {
	if p == nil || p.managementClient == nil {
		return "", 0, nil
	}
	return p.managementClient.GetSheinCookie(storeID)
}

func (p *SheinProcessor) GetSheinStoreCookie(storeID int64) (string, error) {
	if p == nil || p.managementClient == nil {
		return "", nil
	}
	return p.managementClient.GetSheinStoreCookie(storeID)
}

func (p *SheinProcessor) DeleteSheinStoreCookie(storeID int64) (bool, error) {
	if p == nil || p.managementClient == nil {
		return false, nil
	}
	return p.managementClient.DeleteSheinStoreCookie(storeID)
}

func (p *SheinProcessor) SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	if p == nil || p.managementClient == nil {
		return false, nil
	}
	return p.managementClient.SetRuntimeStorePauseStatus(storeID, pause, pauseType)
}

func (p *SheinProcessor) NewManagedAPIClientWithStoreInfo(storeID int64, storeInfo *listingruntime.StoreInfo) *sheinclient.APIClient {
	if p == nil || p.managementClient == nil || storeID <= 0 {
		return nil
	}
	storeService := p.GetRuntimeStoreService()
	return sheinmanagedclient.NewAPIClientWithStoreInfo(
		storeID,
		sheinmanagedclient.NewRuntimeCookieProvider(p, storeService),
		sheinmanagedclient.NewRuntimeStoreConfigProvider(storeService),
		storeInfo,
	)
}

func (p *SheinProcessor) GetImageDownloader() interface {
	DownloadImage(url string) ([]byte, error)
} {
	if p == nil || p.managementClient == nil {
		return nil
	}
	return p.imageDownloader
}

func (p *SheinProcessor) Close(ctx context.Context) {
	p.GetLogger().Info("[SHEIN] closing processor")
	p.CloseBase(ctx)
	p.GetLogger().Info("[SHEIN] processor closed")
}
