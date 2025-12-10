package amazon

import (
	"context"
	"task-processor/common/config"
	"task-processor/common/management"
	"task-processor/common/memory"
	"task-processor/common/processor"
	"task-processor/common/types"
	"task-processor/common/worker"
	"time"

	"github.com/sirupsen/logrus"
)

// AmazonProcessor Amazon平台处理器
type AmazonProcessor struct {
	config           *config.Config
	taskHandler      *TaskHandler
	pipeline         *Pipeline
	managementClient *management.ClientManager
	memoryManager    *memory.MemoryManager
	workerPool       processor.WorkerPool
	logger           *logrus.Logger
}

// NewAmazonProcessor 创建Amazon处理器
func NewAmazonProcessor(cfg *config.Config, logger *logrus.Logger) *AmazonProcessor {
	return NewAmazonProcessorWithManagementClient(cfg, logger, nil)
}

// NewAmazonProcessorWithManagementClient 创建Amazon处理器（使用外部managementClient）
func NewAmazonProcessorWithManagementClient(
	cfg *config.Config,
	logger *logrus.Logger,
	managementClient *management.ClientManager,
) *AmazonProcessor {
	// 如果没有传入managementClient，则创建新的
	if managementClient == nil {
		managementClient = management.NewClientManager(&cfg.Management)
		managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)
	}

	// 创建内存管理器
	memoryManager := memory.NewMemoryManager(managementClient)

	// 设置 ShopPauseManager 的 StoreClient
	storeClient := managementClient.GetStoreClient()
	memoryManager.ShopPauseManager.SetStoreClient(storeClient)

	p := &AmazonProcessor{
		config:           cfg,
		managementClient: managementClient,
		memoryManager:    memoryManager,
		logger:           logger,
	}

	// 创建 WorkerPool
	p.workerPool = worker.NewPool(p, cfg.Worker)

	// 初始化任务处理器
	p.taskHandler = NewTaskHandler(p)

	// 构建处理管道
	p.pipeline = p.buildPipeline()

	return p
}

// SetUserToken 设置用户访问令牌
func (p *AmazonProcessor) SetUserToken(accessToken, tenantID string) {
	if p.managementClient != nil {
		client := p.managementClient.GetClient()
		client.SetUserToken(accessToken, tenantID)
		p.logger.Infof("[Amazon] 已设置用户令牌到管理系统客户端 (租户: %s)", tenantID)
	}
}

// GetManagementClient 获取管理系统客户端
func (p *AmazonProcessor) GetManagementClient() *management.ClientManager {
	return p.managementClient
}

// ProcessTask 处理Amazon任务
func (p *AmazonProcessor) ProcessTask(ctx context.Context, task types.Task) error {
	p.logger.Infof("[Amazon] 开始处理任务: ID=%s, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// 使用任务处理器处理任务
	if err := p.taskHandler.ProcessTask(ctx, task, p.pipeline); err != nil {
		p.logger.Errorf("[Amazon] 处理产品失败: %v", err)
		return err
	}

	p.logger.Infof("[Amazon] 任务处理完成: ID=%s", task.ID)
	return nil
}

// buildPipeline 构建处理管道
func (p *AmazonProcessor) buildPipeline() *Pipeline {
	pipeline := NewPipeline()

	// 注意：由于Go的循环导入限制，Handler的初始化需要在运行时完成
	// 当前管道为空，实际的Handler会在TaskHandler中动态添加

	p.logger.Info("[Amazon] 处理管道已创建（Handler将在运行时添加）")
	return pipeline
}

// Start 启动Amazon处理器
func (p *AmazonProcessor) Start(ctx context.Context) error {
	p.logger.Info("[Amazon] 启动任务处理器")

	// 启动 WorkerPool
	if p.workerPool != nil {
		p.workerPool.Start(ctx)
	}

	p.logger.Info("[Amazon] 任务处理器启动完成")
	return nil
}

// GetWorkerPool 获取工作池
func (p *AmazonProcessor) GetWorkerPool() processor.WorkerPool {
	return p.workerPool
}

// Close 关闭处理器
func (p *AmazonProcessor) Close() {
	p.logger.Info("[Amazon] 关闭任务处理器")

	// 关闭 WorkerPool
	if p.workerPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p.workerPool.Stop(ctx)
	}

	p.logger.Info("[Amazon] 任务处理器已关闭")
}
