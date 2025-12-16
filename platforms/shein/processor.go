package shein

import (
	"context"
	"task-processor/common/management"
	"task-processor/common/management/impl"
	"task-processor/common/memory"
	"task-processor/common/processor"
	shops "task-processor/common/shein"
	"task-processor/common/worker"
	"task-processor/internal/config"
	"task-processor/platforms/shein/modules"
	"time"

	"github.com/sirupsen/logrus"
)

// SheinProcessor SHEIN任务处理器
type SheinProcessor struct {
	config              *config.Config
	memoryManager       *memory.MemoryManager
	managementClientMgr *management.ClientManager
	shopClientMgr       *shops.ClientManager
	reListingProcessor  *modules.ReListingProcessor
	autoPricingHandler  *modules.AutoPricingHandler
	amazonProcessor     interface{} // 共享的Amazon处理器

	// 组件
	workerPool  processor.WorkerPool
	taskHandler *TaskHandler
	pipeline    *Pipeline
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
func NewSheinProcessorWithSharedResources(cfg *config.Config, managementClientMgr *management.ClientManager, amazonProcessor interface{}) *SheinProcessor {
	// 如果没有传入managementClient，则创建新的
	if managementClientMgr == nil {
		managementClientMgr = management.NewClientManager(&cfg.Management)
		// 设置数据新鲜度天数
		managementClientMgr.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)
	}

	memoryManager := memory.NewMemoryManager(managementClientMgr)
	shopClientMgr := shops.NewClientManager(memoryManager.CookieManager)
	reListingProcessor := modules.NewReListingProcessor(memoryManager.ReListingQueue)
	autoPricingHandler := modules.NewAutoPricingHandler(shopClientMgr, managementClientMgr)

	// 初始化全局图片下载客户端管理器
	imageDownloaderManager := management.NewClientManager(&cfg.Management)
	_ = imageDownloaderManager.GetImageDownloader()

	p := &SheinProcessor{
		config:              cfg,
		memoryManager:       memoryManager,
		managementClientMgr: managementClientMgr,
		shopClientMgr:       shopClientMgr,
		reListingProcessor:  reListingProcessor,
		autoPricingHandler:  autoPricingHandler,
		amazonProcessor:     amazonProcessor, // 使用共享的Amazon处理器
	}

	// 设置 ShopPauseManager 的 StoreClient
	storeClient := managementClientMgr.GetStoreClient()
	memoryManager.ShopPauseManager.SetStoreClient(storeClient)

	// 初始化组件 - 使用通用的worker.Pool
	p.workerPool = worker.NewPool(p, cfg.Worker)

	p.taskHandler = NewTaskHandler(cfg, memoryManager, shopClientMgr, managementClientMgr)
	p.pipeline = CreateTaskProcessingPipeline(p, cfg)

	return p
}

// Start 启动任务处理器
func (p *SheinProcessor) Start(ctx context.Context) error {
	logrus.Info("[SHEIN] 启动任务处理器")

	p.workerPool.Start(ctx)

	// 注意：任务获取现在由统一任务获取器 (UnifiedTaskFetcher) 处理
	// 不再在这里启动独立的任务获取器

	go p.processReListingTasksPeriodically(ctx)
	go p.logMetricsPeriodically(ctx)

	// 启动自动核价处理器（如果启用）
	if p.config.AutoPricing.Shein.Enabled {
		autoPricingInterval := time.Duration(p.config.AutoPricing.Shein.Interval) * time.Second
		if autoPricingInterval <= 0 {
			autoPricingInterval = 5 * time.Minute
		}
		logrus.Infof("[SHEIN] 启动自动核价处理器，间隔: %v", autoPricingInterval)
		go p.autoPricingHandler.Start(ctx, autoPricingInterval)
	} else {
		logrus.Info("[SHEIN] 自动核价处理器已禁用")
	}

	return nil
}

// ProcessTask 处理任务（供Worker调用）
func (p *SheinProcessor) ProcessTask(ctx context.Context, task modules.Task) error {
	return p.taskHandler.ProcessTask(ctx, task, p.pipeline)
}

// GetShopClientManager 获取店铺客户端管理器
func (p *SheinProcessor) GetShopClientManager() *shops.ClientManager {
	return p.shopClientMgr
}

// GetMemoryManager 获取内存管理器
func (p *SheinProcessor) GetMemoryManager() *memory.MemoryManager {
	return p.memoryManager
}

// GetManagementClientManager 获取管理客户端管理器
func (p *SheinProcessor) GetManagementClientManager() *management.ClientManager {
	return p.managementClientMgr
}

// GetManagementClient 获取管理系统客户端（用于设置token）
func (p *SheinProcessor) GetManagementClient() *impl.ManagementAPIClientImpl {
	return p.managementClientMgr.GetClient()
}

// GetWorkerPool 获取工作池（供TaskSubmitter使用）
func (p *SheinProcessor) GetWorkerPool() processor.WorkerPool {
	return p.workerPool
}

// Close 关闭处理器
func (p *SheinProcessor) Close() {
	logrus.Info("[SHEIN] 关闭任务处理器")

	if p.workerPool != nil {
		ctx := context.Background()
		p.workerPool.Stop(ctx)
	}

	logrus.Info("[SHEIN] 任务处理器已关闭")
}

// processReListingTasksPeriodically 定期处理重新上架任务
func (p *SheinProcessor) processReListingTasksPeriodically(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("[SHEIN] 重新上架任务处理器停止")
			return
		case <-ticker.C:
			if err := p.reListingProcessor.ProcessReListingTasks(ctx); err != nil {
				logrus.Errorf("[SHEIN] 处理重新上架任务失败: %v", err)
			}
		}
	}
}

// logMetricsPeriodically 定期输出指标统计
func (p *SheinProcessor) logMetricsPeriodically(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("[SHEIN] 指标统计输出器停止")
			// 最后输出一次统计
			if metrics := p.getMetrics(); metrics != nil {
				logrus.Infof("[SHEIN] 最终统计: %+v", metrics)
			}
			return
		case <-ticker.C:
			if metrics := p.getMetrics(); metrics != nil {
				logrus.Infof("[SHEIN] 当前统计: %+v", metrics)
			}
		}
	}
}

// getMetrics 获取指标统计
func (p *SheinProcessor) getMetrics() map[string]interface{} {
	// 这里可以实现具体的指标收集逻辑
	return map[string]interface{}{
		"available_slots": p.workerPool.AvailableSlots(),
	}
}
