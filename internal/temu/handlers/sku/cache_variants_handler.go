package sku

import (
	"fmt"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	domainProduct "task-processor/internal/product"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/pipeline"

	"github.com/sirupsen/logrus"
)

// CacheVariantsHandler 缓存变体数据处理器
// 将已获取的变体数据批量缓存到服务器
type CacheVariantsHandler struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcher
}

// NewCacheVariantsHandler 创建缓存变体数据处理器（支持分布式获取器）
func NewCacheVariantsHandler(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor domainProduct.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *CacheVariantsHandler {
	logger := logrus.WithField("handler", "CacheVariantsHandler")

	// 使用工厂模式创建获取器
	factory := appProduct.NewFetcherFactory()

	// 根据配置创建获取器
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		// 降级到本地获取器
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	return &CacheVariantsHandler{
		logger:  logger,
		fetcher: fetcher,
	}
}

// Name 返回处理器名称
func (h *CacheVariantsHandler) Name() string {
	return "缓存变体数据到服务器"
}

// Handle 处理任务
func (h *CacheVariantsHandler) Handle(ctx pipeline.TaskContext) error {
	// 检查是否有变体数据
	var variants []*model.Product
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		variants = amazonCtx.GetVariants()
	}

	if len(variants) == 0 {
		h.logger.Debug("没有变体数据，跳过缓存步骤")
		return nil
	}

	// 检查任务上下文中的必要数据
	task := ctx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 构建缓存请求
	req := &domainProduct.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.Platform,
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    task.Creator,
	}

	// 缓存变体数据
	if err := h.fetcher.CacheVariants(req, variants); err != nil {
		h.logger.Warnf("⚠️ 缓存变体数据失败: %v", err)
		// 缓存失败不影响主流程，只记录警告
		return nil
	}

	h.logger.Infof("✅ 变体数据已缓存: 数量=%d", len(variants))
	return nil
}


