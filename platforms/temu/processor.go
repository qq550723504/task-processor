package temu

import (
	"context"
	"fmt"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management"
	"task-processor/common/memory"
	"task-processor/common/pipeline"
	"task-processor/common/processor"
	"task-processor/common/types"
	"task-processor/common/worker"
	"task-processor/openai"
	"task-processor/platforms/temu/handlers"
	temuPipeline "task-processor/platforms/temu/pipeline"
	"time"

	"github.com/sirupsen/logrus"
)

// TemuProcessor TEMU平台处理器
type TemuProcessor struct {
	config             *config.Config
	amazonProcessor    *amazon.AmazonProcessor
	taskHandler        *TaskHandler
	pipeline           *pipeline.Pipeline
	managementClient   *management.ClientManager
	memoryManager      *memory.MemoryManager
	workerPool         processor.WorkerPool
	autoPricingHandler *handlers.AutoPricingHandler
	logger             *logrus.Logger
}

// NewTemuProcessor 创建TEMU处理器
func NewTemuProcessor(cfg *config.Config, logger *logrus.Logger) *TemuProcessor {
	return NewTemuProcessorWithManagementClient(cfg, logger, nil)
}

// NewTemuProcessorWithManagementClient 创建TEMU处理器（使用外部managementClient）
func NewTemuProcessorWithManagementClient(cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager) *TemuProcessor {
	// 初始化Amazon处理器（如果启用）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled {
		amazonProcessor = amazon.NewAmazonProcessor(&cfg.Amazon)
		logger.Info("[TEMU] Amazon爬虫已启用")
	}

	// 如果没有传入managementClient，则创建新的
	if managementClient == nil {
		managementClient = management.NewClientManager(&cfg.Management)
		// 设置数据新鲜度天数
		managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)
	}

	// 创建内存管理器
	memoryManager := memory.NewMemoryManager(managementClient)

	// 设置 ShopPauseManager 的 StoreClient
	storeClient := managementClient.GetStoreClient()
	memoryManager.ShopPauseManager.SetStoreClient(storeClient)

	p := &TemuProcessor{
		config:           cfg,
		amazonProcessor:  amazonProcessor,
		managementClient: managementClient,
		memoryManager:    memoryManager,
		logger:           logger,
	}

	// 创建 WorkerPool（内部管理）
	p.workerPool = worker.NewPool(p, cfg.Worker)

	// 初始化任务处理器
	p.taskHandler = NewTaskHandler(p)

	// 初始化自动核价处理器
	p.autoPricingHandler = handlers.NewAutoPricingHandler(managementClient, cfg.Management.StoreIDs)

	// 为TEMU平台确保Amazon爬虫可用（因为TEMU需要处理Amazon产品）
	if !p.config.Amazon.Enabled {
		logger.Info("[TEMU] 自动启用Amazon爬虫以支持Amazon产品处理")
		p.config.Amazon.Enabled = true
		// 设置默认配置
		if p.config.Amazon.PoolSize == 0 {
			p.config.Amazon.PoolSize = 1
		}
		if p.config.Amazon.ViewportWidth == 0 {
			p.config.Amazon.ViewportWidth = 1920
		}
		if p.config.Amazon.ViewportHeight == 0 {
			p.config.Amazon.ViewportHeight = 1080
		}
		p.config.Amazon.Headless = true // TEMU处理器默认使用无头模式
	}

	// 使用统一的管道构建方法
	p.pipeline = p.buildPipeline()

	return p
}

// SetUserToken 设置用户访问令牌
func (p *TemuProcessor) SetUserToken(accessToken, tenantID string) {
	if p.managementClient != nil {
		client := p.managementClient.GetClient()
		client.SetUserToken(accessToken, tenantID)
		p.logger.Infof("[TEMU] 已设置用户令牌到管理系统客户端 (租户: %s)", tenantID)
	}
}

// GetManagementClient 获取管理系统客户端
func (p *TemuProcessor) GetManagementClient() *management.ClientManager {
	return p.managementClient
}

// ProcessTask 处理TEMU任务
func (p *TemuProcessor) ProcessTask(ctx context.Context, task types.Task) error {
	p.logger.Infof("[TEMU] 开始处理任务: ID=%s, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// TEMU特定的任务处理逻辑
	if err := p.processTemuProduct(ctx, task); err != nil {
		p.logger.Errorf("[TEMU] 处理产品失败: %v", err)
		return err
	}

	p.logger.Infof("[TEMU] 任务处理完成: ID=%s", task.ID)
	return nil
}

// processTemuProduct 处理TEMU产品
func (p *TemuProcessor) processTemuProduct(ctx context.Context, task types.Task) error {
	p.logger.Infof("[TEMU] 处理产品: %s", task.ProductID)

	// 创建动态管道
	dynamicPipeline := p.createDynamicPipeline(task)

	// 使用任务处理器处理任务
	if err := p.taskHandler.ProcessTask(ctx, task, dynamicPipeline); err != nil {
		return fmt.Errorf("任务处理失败: %w", err)
	}

	p.logger.Infof("[TEMU] 产品处理完成: %s", task.ProductID)
	return nil
}

// createDynamicPipeline 创建动态管道
func (p *TemuProcessor) createDynamicPipeline(task types.Task) *pipeline.Pipeline {
	// 注意：当前管道构建器不支持基于task的动态配置，task.StoreID等信息会在处理器中使用
	_ = task // 标记task参数已被考虑，避免unused参数警告
	return p.buildPipeline()
}

// buildPipeline 构建管道（统一方法）
func (p *TemuProcessor) buildPipeline() *pipeline.Pipeline {
	// 创建OpenAI客户端配置
	openaiConfig := openai.NewClientConfig(
		p.config.OpenAI.APIKey,
		p.config.OpenAI.Model,
		p.config.OpenAI.BaseURL,
		p.config.OpenAI.Timeout,
	)

	// 使用管道构建器创建管道
	builder := temuPipeline.NewBuilder(
		p.managementClient.GetStoreClient(),                // storeClient
		p.managementClient.GetRawJsonDataClient(),          // rawJsonDataClient
		p.managementClient.GetFilterRuleClient(),           // filterRuleClient
		p.managementClient.GetProfitRuleClient(),           // profitRuleClient
		p.managementClient.GetStoreClient(),                // storeAPIClient
		p.managementClient.GetProductImportMappingClient(), // mappingClient
		p.memoryManager,   // memoryManager
		&p.config.Amazon,  // amazonConfig
		p.amazonProcessor, // amazonProcessor (共享实例)
		openaiConfig,      // openaiConfig
	)
	return builder.Build()
}

// Start 启动TEMU处理器
func (p *TemuProcessor) Start(ctx context.Context) error {
	p.logger.Info("[TEMU] 启动任务处理器")

	// 启动 WorkerPool
	if p.workerPool != nil {
		p.workerPool.Start(ctx)
	}

	// 启动自动核价处理器（如果启用）
	if p.config.AutoPricing.Temu.Enabled {
		autoPricingInterval := time.Duration(p.config.AutoPricing.Temu.Interval) * time.Second
		if autoPricingInterval <= 0 {
			autoPricingInterval = 30 * time.Minute
		}
		p.logger.Infof("[TEMU] 启动自动核价处理器，间隔: %v", autoPricingInterval)
		go p.autoPricingHandler.Start(ctx, autoPricingInterval)
	} else {
		p.logger.Info("[TEMU] 自动核价处理器已禁用")
	}

	p.logger.Info("[TEMU] 任务处理器启动完成")
	return nil
}

// GetWorkerPool 获取工作池（供TaskSubmitter使用）
func (p *TemuProcessor) GetWorkerPool() processor.WorkerPool {
	return p.workerPool
}

// GetAmazonProcessor 获取共享的Amazon处理器
func (p *TemuProcessor) GetAmazonProcessor() any {
	return p.amazonProcessor
}

// Close 关闭处理器
func (p *TemuProcessor) Close() {
	p.logger.Info("[TEMU] 关闭TEMU任务处理器")

	// 关闭 WorkerPool
	if p.workerPool != nil {
		ctx := context.Background()
		p.workerPool.Stop(ctx)
	}

	// 关闭Amazon处理器
	if p.amazonProcessor != nil {
		p.amazonProcessor.Shutdown()
	}

	p.logger.Info("[TEMU] 任务处理器已关闭")
}
