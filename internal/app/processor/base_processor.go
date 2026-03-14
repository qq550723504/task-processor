// Package processor 提供基础处理器实现
package processor

import (
	"context"
	"task-processor/internal/app/state"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// BaseProcessor 基础处理器结构
// 包含所有平台处理器的通用字段和方法
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
	// 如果没有传入managementClient，则创建新的
	managementClient := cfg.ManagementClient
	if managementClient == nil {
		managementClient = management.NewClientManager(&cfg.Config.Management)
		// 设置数据新鲜度天数
		managementClient.SetDataFreshnessDays(cfg.Config.Amazon.DataFreshnessDays)
	}

	// 创建状态管理器 - 使用传入的context用于长期运行的后台任务
	memoryManager := state.NewMemoryManager(ctx, managementClient)

	// 设置 ShopPauseManager 的 StoreClient
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

// GetConfig 获取配置
func (bp *BaseProcessor) GetConfig() *config.Config {
	return bp.config
}

// GetManagementClient 获取管理系统客户端
func (bp *BaseProcessor) GetManagementClient() *management.ClientManager {
	return bp.managementClient
}

// GetMemoryManager 获取状态管理器
func (bp *BaseProcessor) GetMemoryManager() *state.MemoryManager {
	return bp.memoryManager
}

// GetLogger 获取日志器
func (bp *BaseProcessor) GetLogger() *logrus.Logger {
	return bp.logger
}

// GetPlatform 获取平台名称
func (bp *BaseProcessor) GetPlatform() string {
	return bp.platform
}

// SetWorkerPool 设置工作池
func (bp *BaseProcessor) SetWorkerPool(pool worker.WorkerPool) {
	bp.workerPool = pool
}

// GetWorkerPool 获取工作池
func (bp *BaseProcessor) GetWorkerPool() worker.WorkerPool {
	return bp.workerPool
}

// StartBase 基础启动逻辑
func (bp *BaseProcessor) StartBase(ctx context.Context) error {
	log := logger.GetGlobalLogger("worker.base_processor")
	log.WithField(logger.FieldPlatform, bp.platform).Info("启动基础处理器组件")

	// 启动 WorkerPool
	if bp.workerPool != nil {
		bp.workerPool.Start(ctx)
	}

	log.WithField(logger.FieldPlatform, bp.platform).Info("基础处理器组件启动完成")
	return nil
}

// CloseBase 基础关闭逻辑
func (bp *BaseProcessor) CloseBase(ctx context.Context) {
	log := logger.GetGlobalLogger("worker.base_processor")
	log.WithField(logger.FieldPlatform, bp.platform).Info("关闭基础处理器组件")

	// 关闭 WorkerPool
	if bp.workerPool != nil {
		bp.workerPool.Stop(ctx)
	}

	log.WithField(logger.FieldPlatform, bp.platform).Info("基础处理器组件已关闭")
}
