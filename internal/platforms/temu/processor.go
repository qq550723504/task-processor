package temu

import (
	"context"
	"fmt"
	"task-processor/internal/clients/openai"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/management"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/common/processor"
	"task-processor/internal/common/types"
	"task-processor/internal/common/worker"
	"task-processor/internal/config"
	"task-processor/internal/platforms/temu/handlers"
	temuPipeline "task-processor/internal/platforms/temu/pipeline"
	"time"

	"github.com/sirupsen/logrus"
)

// TemuProcessor TEMU平台处理器
type TemuProcessor struct {
	*processor.BaseProcessor                              // 继承基础处理器
	amazonProcessor          *amazon.AmazonProcessor      // TEMU特定：Amazon处理器
	taskHandler              *TaskHandler                 // TEMU特定：任务处理器
	pipeline                 *pipeline.Pipeline           // TEMU特定：处理管道
	autoPricingHandler       *handlers.AutoPricingHandler // TEMU特定：自动核价处理器
}

// NewTemuProcessor 创建TEMU处理器
func NewTemuProcessor(cfg *config.Config, logger *logrus.Logger) *TemuProcessor {
	return NewTemuProcessorWithManagementClient(cfg, logger, nil)
}

// NewTemuProcessorWithManagementClient 创建TEMU处理器（使用外部managementClient）
func NewTemuProcessorWithManagementClient(cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager) *TemuProcessor {
	// 创建基础处理器
	baseProcessor := processor.NewBaseProcessor(&processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: managementClient,
		Logger:           logger,
		Platform:         "TEMU",
	})

	// 初始化Amazon处理器（如果启用）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled {
		amazonProcessor = amazon.NewAmazonProcessor(&cfg.Amazon)
		logger.Info("[TEMU] Amazon爬虫已启用")
	}

	p := &TemuProcessor{
		BaseProcessor:   baseProcessor,
		amazonProcessor: amazonProcessor,
	}

	// 创建 WorkerPool（内部管理）
	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)

	// 初始化任务处理器
	p.taskHandler = NewTaskHandler(p)

	// 初始化自动核价处理器
	p.autoPricingHandler = handlers.NewAutoPricingHandler(p.GetManagementClient(), cfg.Management.StoreIDs)

	// 为TEMU平台确保Amazon爬虫可用（因为TEMU需要处理Amazon产品）
	if !cfg.Amazon.Enabled {
		logger.Info("[TEMU] 自动启用Amazon爬虫以支持Amazon产品处理")
		cfg.Amazon.Enabled = true
		// 设置默认配置
		if cfg.Amazon.PoolSize == 0 {
			cfg.Amazon.PoolSize = 1
		}
		if cfg.Amazon.ViewportWidth == 0 {
			cfg.Amazon.ViewportWidth = 1920
		}
		if cfg.Amazon.ViewportHeight == 0 {
			cfg.Amazon.ViewportHeight = 1080
		}
		cfg.Amazon.Headless = true // TEMU处理器默认使用无头模式
	}

	// 使用统一的管道构建方法
	p.pipeline = p.buildPipeline()

	return p
}

// NewTemuProcessorWithSharedAmazon 创建TEMU处理器（使用共享Amazon处理器）
func NewTemuProcessorWithSharedAmazon(cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedAmazonProcessor *amazon.AmazonProcessor) *TemuProcessor {
	// 创建基础处理器
	baseProcessor := processor.NewBaseProcessor(&processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: managementClient,
		Logger:           logger,
		Platform:         "TEMU",
	})

	// 使用共享的Amazon处理器
	var amazonProcessor *amazon.AmazonProcessor
	if sharedAmazonProcessor != nil {
		amazonProcessor = sharedAmazonProcessor
		logger.Info("[TEMU] 使用共享的Amazon爬虫实例")
	} else if cfg.Amazon.Enabled {
		amazonProcessor = amazon.NewAmazonProcessor(&cfg.Amazon)
		logger.Info("[TEMU] Amazon爬虫已启用")
	}

	p := &TemuProcessor{
		BaseProcessor:   baseProcessor,
		amazonProcessor: amazonProcessor,
	}

	// 创建 WorkerPool（内部管理）
	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)

	// 初始化任务处理器
	p.taskHandler = NewTaskHandler(p)

	// 初始化自动核价处理器
	p.autoPricingHandler = handlers.NewAutoPricingHandler(p.GetManagementClient(), cfg.Management.StoreIDs)

	// 使用统一的管道构建方法
	p.pipeline = p.buildPipeline()

	return p
}

// SetUserToken 设置用户访问令牌（重写基类方法以添加TEMU特定日志）
func (p *TemuProcessor) SetUserToken(accessToken, tenantID string) {
	// 调用基类方法
	p.BaseProcessor.SetUserToken(accessToken, tenantID)
	p.GetLogger().Infof("[TEMU] 已设置用户令牌到管理系统客户端 (租户: %s)", tenantID)
}

// ProcessTask 处理TEMU任务
func (p *TemuProcessor) ProcessTask(ctx context.Context, task *types.Task) error {
	logger := p.GetLogger()
	logger.Infof("[TEMU] 开始处理任务: ID=%s, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// TEMU特定的任务处理逻辑
	if err := p.processTemuProduct(ctx, *task); err != nil {
		logger.Errorf("[TEMU] 处理产品失败: %v", err)
		return err
	}

	logger.Infof("[TEMU] 任务处理完成: ID=%s", task.ID)
	return nil
}

// processTemuProduct 处理TEMU产品
func (p *TemuProcessor) processTemuProduct(ctx context.Context, task types.Task) error {
	logger := p.GetLogger()
	logger.Infof("[TEMU] 处理产品: %s", task.ProductID)

	// 使用任务处理器处理任务
	if err := p.taskHandler.ProcessTask(ctx, task, p.pipeline); err != nil {
		return fmt.Errorf("任务处理失败: %w", err)
	}

	logger.Infof("[TEMU] 产品处理完成: %s", task.ProductID)
	return nil
}

// buildPipeline 构建管道（统一方法）
func (p *TemuProcessor) buildPipeline() *pipeline.Pipeline {
	config := p.GetConfig()
	managementClient := p.GetManagementClient()
	memoryManager := p.GetMemoryManager()

	// 创建OpenAI客户端配置
	openaiConfig := openai.NewClientConfig(
		config.OpenAI.APIKey,
		config.OpenAI.Model,
		config.OpenAI.BaseURL,
		config.OpenAI.Timeout,
	)

	// 使用管道构建器创建管道
	builder := temuPipeline.NewBuilder(
		managementClient.GetStoreClient(),                // storeClient
		managementClient.GetRawJsonDataClient(),          // rawJsonDataClient
		managementClient.GetFilterRuleClient(),           // filterRuleClient
		managementClient.GetProfitRuleClient(),           // profitRuleClient
		managementClient.GetStoreClient(),                // storeAPIClient
		managementClient.GetProductImportMappingClient(), // mappingClient
		memoryManager,     // memoryManager
		&config.Amazon,    // amazonConfig
		p.amazonProcessor, // amazonProcessor (共享实例)
		openaiConfig,      // openaiConfig
	)
	return builder.Build()
}

// Start 启动TEMU处理器
func (p *TemuProcessor) Start(ctx context.Context) error {
	// 启动基础组件
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	// 启动自动核价处理器（如果启用）
	config := p.GetConfig()
	if config.Platforms.Temu.AutoPricing.Enabled {
		autoPricingInterval := time.Duration(config.Platforms.Temu.AutoPricing.Interval) * time.Second
		if autoPricingInterval <= 0 {
			autoPricingInterval = 30 * time.Minute
		}
		p.GetLogger().Infof("[TEMU] 启动自动核价处理器，间隔: %v", autoPricingInterval)
		go p.autoPricingHandler.Start(ctx, autoPricingInterval)
	} else {
		p.GetLogger().Info("[TEMU] 自动核价处理器已禁用")
	}

	p.GetLogger().Info("[TEMU] 任务处理器启动完成")
	return nil
}

// GetAmazonProcessor 获取共享的Amazon处理器
func (p *TemuProcessor) GetAmazonProcessor() any {
	return p.amazonProcessor
}

// Close 关闭处理器
func (p *TemuProcessor) Close() {
	p.GetLogger().Info("[TEMU] 关闭TEMU任务处理器")

	// 关闭基础组件
	p.CloseBase()

	// 关闭Amazon处理器
	if p.amazonProcessor != nil {
		p.amazonProcessor.Shutdown()
	}

	p.GetLogger().Info("[TEMU] 任务处理器已关闭")
}
