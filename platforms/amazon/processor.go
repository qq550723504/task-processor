// Package amazon 提供Amazon平台主处理器
package amazon

import (
	"context"
	"task-processor/common/management"
	"task-processor/common/memory"
	"task-processor/common/processor"
	"task-processor/common/types"
	"task-processor/common/worker"
	"task-processor/internal/amazon/service"
	"task-processor/internal/config"
	"task-processor/platforms/amazon/api"
	"task-processor/platforms/amazon/internal/model"
	amazonService "task-processor/platforms/amazon/internal/service"
	"time"

	"github.com/sirupsen/logrus"
)

// Processor Amazon平台处理器
type Processor struct {
	config           *config.Config
	services         *model.Services
	pipelineService  *amazonService.PipelineService
	managementClient *management.ClientManager
	memoryManager    *memory.MemoryManager
	workerPool       processor.WorkerPool
	logger           *logrus.Logger
}

// NewProcessor 创建Amazon处理器
func NewProcessor(cfg *config.Config, logger *logrus.Logger) *Processor {
	return NewProcessorWithManagementClient(cfg, logger, nil)
}

// NewProcessorWithManagementClient 创建Amazon处理器（使用外部managementClient）
func NewProcessorWithManagementClient(
	cfg *config.Config,
	logger *logrus.Logger,
	managementClient *management.ClientManager,
) *Processor {
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

	// 创建服务集合
	services := model.NewServices()
	services.SetManagementClient(managementClient)
	services.SetMemoryManager(memoryManager)

	// 创建 API 客户端
	apiClient := createAPIClient(cfg)
	services.SetAPIClient(apiClient)

	// 创建产品类型缓存服务
	productTypeCache := service.NewProductTypeCache(apiClient, "cache/product_types")
	services.SetProductTypeCache(productTypeCache)

	p := &Processor{
		config:           cfg,
		services:         services,
		managementClient: managementClient,
		memoryManager:    memoryManager,
		logger:           logger,
	}

	// 创建 WorkerPool
	p.workerPool = worker.NewPool(p, cfg.Worker)

	// 构建处理管道
	p.buildPipeline()

	return p
}

// createAPIClient 创建 Amazon SP-API 客户端
func createAPIClient(cfg *config.Config) *api.Client {
	spapi := cfg.Amazon.SPAPI
	marketplaceID := spapi.DefaultMarketplace
	if marketplaceID == "" {
		marketplaceID = spapi.MarketplaceID
	}

	sellerID := spapi.SellerID
	if m, ok := spapi.Marketplaces[marketplaceID]; ok && m.SellerID != "" {
		sellerID = m.SellerID
	}

	apiCfg := &api.Config{
		Region:         spapi.Region,
		MarketplaceID:  marketplaceID,
		SellerID:       sellerID,
		ClientID:       spapi.ClientID,
		ClientSecret:   spapi.ClientSecret,
		RefreshToken:   spapi.RefreshToken,
		AWSAccessKeyID: spapi.AWSAccessKeyID,
		AWSSecretKey:   spapi.AWSSecretKey,
	}

	return api.NewClient(apiCfg)
}

// buildPipeline 构建处理管道
func (p *Processor) buildPipeline() {
	builder := amazonService.NewPipelineBuilder(p.services)
	p.pipelineService = builder.BuildAmazonPipeline()

	p.logger.Infof("[Amazon] 处理管道已构建，共 %d 个步骤", p.pipelineService.GetHandlerCount())
}

// ProcessTask 处理Amazon任务
func (p *Processor) ProcessTask(ctx context.Context, task types.Task) error {
	p.logger.Infof("[Amazon] 开始处理任务: ID=%s, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// 创建任务数据
	data := map[string]interface{}{
		"task_id":    task.ID,
		"product_id": task.ProductID,
		"store_id":   task.StoreID,
		"tenant_id":  task.TenantID,
		"context":    ctx,
	}

	// 执行管道处理
	if err := p.pipelineService.Execute(p.services, data); err != nil {
		p.logger.Errorf("[Amazon] 处理产品失败: %v", err)
		return err
	}

	p.logger.Infof("[Amazon] 任务处理完成: ID=%s", task.ID)
	return nil
}

// Start 启动Amazon处理器
func (p *Processor) Start(ctx context.Context) error {
	p.logger.Info("[Amazon] 启动任务处理器")

	// 启动 WorkerPool
	if p.workerPool != nil {
		p.workerPool.Start(ctx)
	}

	p.logger.Info("[Amazon] 任务处理器启动完成")
	return nil
}

// GetWorkerPool 获取工作池
func (p *Processor) GetWorkerPool() processor.WorkerPool {
	return p.workerPool
}

// Close 关闭处理器
func (p *Processor) Close() {
	p.logger.Info("[Amazon] 关闭任务处理器")

	// 关闭 WorkerPool
	if p.workerPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p.workerPool.Stop(ctx)
	}

	p.logger.Info("[Amazon] 任务处理器已关闭")
}
