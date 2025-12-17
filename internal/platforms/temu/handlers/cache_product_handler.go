package handlers

import (
	"fmt"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/common/product"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// CacheProductHandler 缓存产品数据处理器
// 将已获取的产品数据缓存到服务器
type CacheProductHandler struct {
	logger  *logrus.Entry
	fetcher *product.ProductFetcher
}

// NewCacheProductHandler 创建缓存产品数据处理器
func NewCacheProductHandler(rawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error)
}, amazonConfig *config.AmazonConfig, amazonProcessor interface{}) *CacheProductHandler {

	// 提取Amazon处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
		}
	}

	return &CacheProductHandler{
		logger:  logrus.WithField("handler", "CacheProductHandler"),
		fetcher: product.NewProductFetcher(rawJsonDataClient, amazonConfig, ap),
	}
}

// Name 返回处理器名称
func (h *CacheProductHandler) Name() string {
	return "缓存产品数据到服务器"
}

// Handle 处理任务
func (h *CacheProductHandler) Handle(ctx *pipeline.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.AmazonProduct == nil {
		h.logger.Warn("产品数据未获取，跳过缓存步骤")
		return nil
	}

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	// 构建缓存请求
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	// 缓存产品数据
	if err := h.fetcher.CacheProduct(req, ctx.AmazonProduct); err != nil {
		h.logger.Warnf("⚠️ 缓存产品数据失败: %v", err)
		// 缓存失败不影响主流程，只记录警告
		return nil
	}

	h.logger.Infof("✅ 产品数据已缓存: ProductID=%s", ctx.Task.ProductID)
	return nil
}
