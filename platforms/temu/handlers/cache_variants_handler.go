package handlers

import (
	"fmt"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"
	"task-processor/common/product"

	"github.com/sirupsen/logrus"
)

// CacheVariantsHandler 缓存变体数据处理器
// 将已获取的变体数据批量缓存到服务器
type CacheVariantsHandler struct {
	logger  *logrus.Entry
	fetcher *product.ProductFetcher
}

// NewCacheVariantsHandler 创建缓存变体数据处理器
func NewCacheVariantsHandler(rawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error)
}, amazonConfig *config.AmazonConfig, amazonProcessor interface{}) *CacheVariantsHandler {

	// 提取Amazon处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
		}
	}

	return &CacheVariantsHandler{
		logger:  logrus.WithField("handler", "CacheVariantsHandler"),
		fetcher: product.NewProductFetcher(rawJsonDataClient, amazonConfig, ap),
	}
}

// Name 返回处理器名称
func (h *CacheVariantsHandler) Name() string {
	return "缓存变体数据到服务器"
}

// Handle 处理任务
func (h *CacheVariantsHandler) Handle(ctx *pipeline.TaskContext) error {
	// 检查是否有变体数据
	if len(ctx.AmazonVariants) == 0 {
		h.logger.Debug("没有变体数据，跳过缓存步骤")
		return nil
	}

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
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

	// 缓存变体数据
	if err := h.fetcher.CacheVariants(req, ctx.AmazonVariants); err != nil {
		h.logger.Warnf("⚠️ 缓存变体数据失败: %v", err)
		// 缓存失败不影响主流程，只记录警告
		return nil
	}

	h.logger.Infof("✅ 变体数据已缓存: 数量=%d", len(ctx.AmazonVariants))
	return nil
}
