package processor

import (
	"context"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"

	"github.com/sirupsen/logrus"
)

// BaseProcessor 基础处理器结构
type BaseProcessor struct {
	config            *config.Config
	storeAPI          listingadmin.StoreAPI
	taskStatusRuntime taskstatus.RuntimeWithTaskRPC
	memoryManager     *state.MemoryManager
	workerPool        worker.WorkerPool
	logger            *logrus.Logger
	platform          string
}

// BaseProcessorConfig 基础处理器配置
type BaseProcessorConfig struct {
	Config                   *config.Config
	StoreAPI                 listingadmin.StoreAPI
	TaskStatusRuntime        taskstatus.RuntimeWithTaskRPC
	DailyCountClientProvider state.DailyCountClientProvider
	Logger                   *logrus.Logger
	Platform                 string
}

// NewBaseProcessor 创建基础处理器
func NewBaseProcessor(ctx context.Context, cfg *BaseProcessorConfig) *BaseProcessor {
	memoryManager := state.NewMemoryManager(ctx, cfg.DailyCountClientProvider)
	if cfg.StoreAPI != nil {
		memoryManager.ShopPauseManager.SetStoreClient(cfg.StoreAPI)
	}

	return &BaseProcessor{
		config:            cfg.Config,
		storeAPI:          cfg.StoreAPI,
		taskStatusRuntime: cfg.TaskStatusRuntime,
		memoryManager:     memoryManager,
		logger:            cfg.Logger,
		platform:          cfg.Platform,
	}
}

func NewBaseProcessorWithMemoryManager(cfg *BaseProcessorConfig, memoryManager *state.MemoryManager) *BaseProcessor {
	return &BaseProcessor{
		config:            cfg.Config,
		storeAPI:          cfg.StoreAPI,
		taskStatusRuntime: cfg.TaskStatusRuntime,
		memoryManager:     memoryManager,
		logger:            cfg.Logger,
		platform:          cfg.Platform,
	}
}

func (bp *BaseProcessor) GetConfig() *config.Config {
	return bp.config
}

func (bp *BaseProcessor) GetStoreAPI() listingadmin.StoreAPI {
	if bp == nil {
		return nil
	}
	return bp.storeAPI
}

func (bp *BaseProcessor) GetTaskStatusRuntime() taskstatus.RuntimeWithTaskRPC {
	if bp == nil {
		return nil
	}
	return bp.taskStatusRuntime
}

func (bp *BaseProcessor) GetMemoryManager() *state.MemoryManager {
	return bp.memoryManager
}

func (bp *BaseProcessor) GetLogger() *logrus.Logger {
	return bp.logger
}

func (bp *BaseProcessor) GetPlatform() string {
	return bp.platform
}

func (bp *BaseProcessor) SetWorkerPool(pool worker.WorkerPool) {
	bp.workerPool = pool
}

func (bp *BaseProcessor) GetWorkerPool() worker.WorkerPool {
	return bp.workerPool
}

func (bp *BaseProcessor) StartBase(ctx context.Context) error {
	log := logger.GetGlobalLogger("worker.base_processor")
	log.WithField(logger.FieldPlatform, bp.platform).Info("启动基础处理器组件")

	if bp.workerPool != nil {
		bp.workerPool.Start(ctx)
	}

	log.WithField(logger.FieldPlatform, bp.platform).Info("基础处理器组件启动完成")
	return nil
}

func (bp *BaseProcessor) CloseBase(ctx context.Context) {
	log := logger.GetGlobalLogger("worker.base_processor")
	log.WithField(logger.FieldPlatform, bp.platform).Info("关闭基础处理器组件")

	if bp.workerPool != nil {
		bp.workerPool.Stop(ctx)
	}

	log.WithField(logger.FieldPlatform, bp.platform).Info("基础处理器组件已关闭")
}
