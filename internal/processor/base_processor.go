package processor

import (
	"context"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/worker"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"

	"github.com/sirupsen/logrus"
)

// BaseProcessor 基础处理器结构
type BaseProcessor struct {
	config           *config.Config
	managementClient *management.ClientManager
	memoryManager    *state.MemoryManager
	workerPool       worker.WorkerPool
	logger           *logrus.Logger
	platform         string
}

// BaseProcessorConfig 基础处理器配置
type BaseProcessorConfig struct {
	Config           *config.Config
	ManagementClient *management.ClientManager
	Logger           *logrus.Logger
	Platform         string
}

// NewBaseProcessor 创建基础处理器
func NewBaseProcessor(ctx context.Context, cfg *BaseProcessorConfig) *BaseProcessor {
	managementClient := cfg.ManagementClient
	if managementClient == nil {
		managementClient = management.NewClientManager(&cfg.Config.Management)
		managementClient.SetDataFreshnessDays(cfg.Config.Amazon.DataFreshnessDays)
		if provider, err := management.NewLocalDataProvider(cfg.Config.Database, cfg.Config.Redis); err != nil {
			cfg.Logger.WithError(err).Warn("failed to configure local management data provider")
		} else if provider != nil {
			managementClient.SetLocalDataProvider(provider)
		}
		cookieRedis := cfg.Config.EffectiveSheinCookieRedis()
		if strings.TrimSpace(cookieRedis.Host) != "" {
			if err := managementClient.SetSheinCookieRedisConfig(&cookieRedis); err != nil {
				cfg.Logger.WithError(err).Warn("failed to configure SHEIN cookie Redis provider")
			}
		}
	}

	memoryManager := state.NewMemoryManager(ctx, managementClient)
	storeClient := managementClient.GetStoreClient()
	memoryManager.ShopPauseManager.SetStoreClient(storeClient)

	return &BaseProcessor{
		config:           cfg.Config,
		managementClient: managementClient,
		memoryManager:    memoryManager,
		logger:           cfg.Logger,
		platform:         cfg.Platform,
	}
}

func NewBaseProcessorWithMemoryManager(cfg *BaseProcessorConfig, memoryManager *state.MemoryManager) *BaseProcessor {
	return &BaseProcessor{
		config:        cfg.Config,
		memoryManager: memoryManager,
		logger:        cfg.Logger,
		platform:      cfg.Platform,
	}
}

func (bp *BaseProcessor) GetConfig() *config.Config {
	return bp.config
}

func (bp *BaseProcessor) GetStoreAPI() managementapi.StoreAPI {
	if bp == nil || bp.managementClient == nil {
		return nil
	}
	return bp.managementClient.GetStoreClient()
}

func (bp *BaseProcessor) GetTaskStatusRuntime() taskstatus.RuntimeWithTaskRPC {
	if bp == nil || bp.managementClient == nil {
		return nil
	}
	return management.NewTaskStatusRuntime(bp.managementClient)
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
