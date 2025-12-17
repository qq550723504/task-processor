// Package processor 提供基础处理器实现
package processor

import (
	"context"
	"task-processor/internal/common/management"
	"task-processor/internal/common/memory"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// BaseProcessor 基础处理器结构
// 包含所有平台处理器的通用字段和方法
type BaseProcessor struct {
	config           *config.Config
	managementClient *management.ClientManager
	memoryManager    *memory.MemoryManager
	workerPool       WorkerPool
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
func NewBaseProcessor(cfg *BaseProcessorConfig) *BaseProcessor {
	// 如果没有传入managementClient，则创建新的
	managementClient := cfg.ManagementClient
	if managementClient == nil {
		managementClient = management.NewClientManager(&cfg.Config.Management)
		// 设置数据新鲜度天数
		managementClient.SetDataFreshnessDays(cfg.Config.Amazon.DataFreshnessDays)
	}

	// 创建内存管理器
	memoryManager := memory.NewMemoryManager(managementClient)

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

// GetMemoryManager 获取内存管理器
func (bp *BaseProcessor) GetMemoryManager() *memory.MemoryManager {
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
func (bp *BaseProcessor) SetWorkerPool(pool WorkerPool) {
	bp.workerPool = pool
}

// GetWorkerPool 获取工作池
func (bp *BaseProcessor) GetWorkerPool() WorkerPool {
	return bp.workerPool
}

// SetUserToken 设置用户访问令牌
func (bp *BaseProcessor) SetUserToken(accessToken, tenantID string) {
	if bp.managementClient != nil {
		client := bp.managementClient.GetClient()
		client.SetUserToken(accessToken, tenantID)
		bp.logger.Infof("[%s] 已设置用户令牌到管理系统客户端 (租户: %s)", bp.platform, tenantID)
	}
}

// StartBase 基础启动逻辑
func (bp *BaseProcessor) StartBase(ctx context.Context) error {
	bp.logger.Infof("[%s] 启动基础处理器组件", bp.platform)

	// 启动 WorkerPool
	if bp.workerPool != nil {
		bp.workerPool.Start(ctx)
	}

	bp.logger.Infof("[%s] 基础处理器组件启动完成", bp.platform)
	return nil
}

// CloseBase 基础关闭逻辑
func (bp *BaseProcessor) CloseBase() {
	bp.logger.Infof("[%s] 关闭基础处理器组件", bp.platform)

	// 关闭 WorkerPool
	if bp.workerPool != nil {
		ctx := context.Background()
		bp.workerPool.Stop(ctx)
	}

	bp.logger.Infof("[%s] 基础处理器组件已关闭", bp.platform)
}
