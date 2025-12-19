package shein

import (
	"context"
	"task-processor/internal/common/management"
	"task-processor/internal/common/management/impl"
	"task-processor/internal/common/processor"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/common/types"
	"task-processor/internal/common/worker"
	"task-processor/internal/config"
	"task-processor/internal/platforms/shein/modules"
	"time"

	"github.com/sirupsen/logrus"
)

// SheinProcessor SHEIN任务处理器
type SheinProcessor struct {
	*processor.BaseProcessor                             // 继承基础处理器
	shopClientMgr            *shops.ClientManager        // SHEIN特定：店铺客户端管理器
	reListingProcessor       *modules.ReListingProcessor // SHEIN特定：重新上架处理器
	autoPricingHandler       *modules.AutoPricingHandler // SHEIN特定：自动核价处理器
	amazonProcessor          any                         // SHEIN特定：共享的Amazon处理器
	taskHandler              *TaskHandler                // SHEIN特定：任务处理器
	pipeline                 *Pipeline                   // SHEIN特定：处理管道
}

// NewSheinProcessor 创建新的SHEIN任务处理器
func NewSheinProcessor(cfg *config.Config) *SheinProcessor {
	return NewSheinProcessorWithManagementClient(cfg, nil)
}

// NewSheinProcessorWithManagementClient 创建新的SHEIN任务处理器（使用外部managementClient）
func NewSheinProcessorWithManagementClient(cfg *config.Config, managementClientMgr *management.ClientManager) *SheinProcessor {
	return NewSheinProcessorWithSharedResources(cfg, managementClientMgr, nil)
}

// NewSheinProcessorWithSharedResources 创建新的SHEIN任务处理器（使用共享资源）
func NewSheinProcessorWithSharedResources(cfg *config.Config, managementClientMgr *management.ClientManager, amazonProcessor any) *SheinProcessor {
	// 创建基础处理器
	baseProcessor := processor.NewBaseProcessor(&processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: managementClientMgr,
		Logger:           logrus.StandardLogger(),
		Platform:         "SHEIN",
	})

	// 获取内存管理器
	memoryManager := baseProcessor.GetMemoryManager()

	// 创建SHEIN特定组件
	shopClientMgr := shops.NewClientManager(memoryManager.CookieManager)
	reListingProcessor := modules.NewReListingProcessor(memoryManager.ReListingQueue)
	autoPricingHandler := modules.NewAutoPricingHandler(shopClientMgr, baseProcessor.GetManagementClient())

	// 初始化全局图片下载客户端管理器
	imageDownloaderManager := management.NewClientManager(&cfg.Management)
	_ = imageDownloaderManager.GetImageDownloader()

	p := &SheinProcessor{
		BaseProcessor:      baseProcessor,
		shopClientMgr:      shopClientMgr,
		reListingProcessor: reListingProcessor,
		autoPricingHandler: autoPricingHandler,
		amazonProcessor:    amazonProcessor,
	}

	// 创建 WorkerPool
	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)

	// 初始化SHEIN特定组件
	p.taskHandler = NewTaskHandler(cfg, memoryManager, shopClientMgr, baseProcessor.GetManagementClient())
	p.pipeline = CreateTaskProcessingPipeline(p, cfg)

	return p
}

// Start 启动任务处理器
func (p *SheinProcessor) Start(ctx context.Context) error {
	// 启动基础组件
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	// 启动SHEIN特定的定期任务
	go p.processReListingTasksPeriodically(ctx)
	go p.logMetricsPeriodically(ctx)

	// 启动自动核价处理器（如果启用）
	config := p.GetConfig()
	if config.Platforms.Shein.AutoPricing.Enabled {
		autoPricingInterval := time.Duration(config.Platforms.Shein.AutoPricing.Interval) * time.Second
		if autoPricingInterval <= 0 {
			autoPricingInterval = 5 * time.Minute
		}
		p.GetLogger().Infof("[SHEIN] 启动自动核价处理器，间隔: %v", autoPricingInterval)
		go p.autoPricingHandler.Start(ctx, autoPricingInterval)
	} else {
		p.GetLogger().Info("[SHEIN] 自动核价处理器已禁用")
	}

	return nil
}

// ProcessTask 处理任务（供Worker调用）
func (p *SheinProcessor) ProcessTask(ctx context.Context, task *types.Task) error {
	return p.taskHandler.ProcessTask(ctx, *task, p.pipeline)
}

// GetShopClientManager 获取店铺客户端管理器
func (p *SheinProcessor) GetShopClientManager() *shops.ClientManager {
	return p.shopClientMgr
}

// GetManagementClientImpl 获取管理系统客户端实现（用于设置token）
func (p *SheinProcessor) GetManagementClientImpl() *impl.ManagementAPIClientImpl {
	return p.GetManagementClient().GetClient()
}

// Close 关闭处理器
func (p *SheinProcessor) Close() {
	p.GetLogger().Info("[SHEIN] 关闭任务处理器")

	// 关闭基础组件
	p.CloseBase()

	p.GetLogger().Info("[SHEIN] 任务处理器已关闭")
}

// processReListingTasksPeriodically 定期处理重新上架任务
func (p *SheinProcessor) processReListingTasksPeriodically(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	logger := p.GetLogger()
	for {
		select {
		case <-ctx.Done():
			logger.Info("[SHEIN] 重新上架任务处理器停止")
			return
		case <-ticker.C:
			if err := p.reListingProcessor.ProcessReListingTasks(ctx); err != nil {
				logger.Errorf("[SHEIN] 处理重新上架任务失败: %v", err)
			}
		}
	}
}

// logMetricsPeriodically 定期输出指标统计
func (p *SheinProcessor) logMetricsPeriodically(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	logger := p.GetLogger()
	for {
		select {
		case <-ctx.Done():
			logger.Info("[SHEIN] 指标统计输出器停止")
			// 最后输出一次统计
			if metrics := p.getMetrics(); metrics != nil {
				logger.Infof("[SHEIN] 最终统计: %+v", metrics)
			}
			return
		case <-ticker.C:
			if metrics := p.getMetrics(); metrics != nil {
				logger.Infof("[SHEIN] 当前统计: %+v", metrics)
			}
		}
	}
}

// getMetrics 获取指标统计
func (p *SheinProcessor) getMetrics() map[string]any {
	// 这里可以实现具体的指标收集逻辑
	return map[string]any{
		"available_slots": p.GetWorkerPool().AvailableSlots(),
	}
}
