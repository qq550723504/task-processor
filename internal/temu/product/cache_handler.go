package product

import (
	"fmt"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	domainProduct "task-processor/internal/product"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// CacheProductHandler 缓存产品数据处理器
// 将已获取的产品数据缓存到服务器
type CacheProductHandler struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcher
}

// NewCacheProductHandler 创建缓存产品数据处理器（支持分布式获取器）
func NewCacheProductHandler(
	fetcher appProduct.ProductFetcher,
) *CacheProductHandler {
	logger := logger.GetGlobalLogger("CacheProductHandler")

	return &CacheProductHandler{
		logger:  logger,
		fetcher: fetcher,
	}
}

// Name 返回处理器名称
func (h *CacheProductHandler) Name() string {
	return "缓存产品数据到服务器"
}

// Handle 处理任务
func (h *CacheProductHandler) Handle(ctx pipeline.TaskContext) error {
	// 检查是否已获取产品数据
	var amazonProduct *model.Product
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		amazonProduct = amazonCtx.GetAmazonProduct()
	}

	if amazonProduct == nil {
		h.logger.Warn("产品数据未获取，跳过缓存步骤")
		return nil
	}

	// 检查任务上下文中的必要数据
	task := ctx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
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

	// 缓存产品数据
	if err := h.fetcher.CacheProduct(req, amazonProduct); err != nil {
		h.logger.Warnf("⚠️ 缓存产品数据失败: %v", err)
		// 缓存失败不影响主流程，只记录警告
		return nil
	}

	h.logger.Infof("✅ 产品数据已缓存: ProductID=%s", task.ProductID)
	return nil
}
