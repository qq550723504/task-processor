package handlers

import (
	"errors"
	"fmt"
	"task-processor/common/amazon"
	"task-processor/common/amazon/model"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"
	"task-processor/common/product"
	"task-processor/internal/config"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandlerV2 原始JSON数据处理器V2（使用公共ProductFetcher）
type RawJsonDataHandlerV2 struct {
	logger  *logrus.Entry
	fetcher *product.ProductFetcher
}

// NewRawJsonDataHandlerV2 创建新的原始JSON数据处理器V2
func NewRawJsonDataHandlerV2(rawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error)
}, amazonConfig *config.AmazonConfig, amazonProcessor interface{}) *RawJsonDataHandlerV2 {

	// 提取Amazon处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
			logrus.Info("[TEMU] 使用共享的Amazon爬虫实例")
		}
	} else if amazonConfig != nil && amazonConfig.Enabled {
		// 如果没有提供共享实例，则创建新的（向后兼容）
		ap = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("[TEMU] Amazon爬虫已启用")
	}

	return &RawJsonDataHandlerV2{
		logger:  logrus.WithField("handler", "RawJsonDataHandlerV2"),
		fetcher: product.NewProductFetcher(rawJsonDataClient, amazonConfig, ap),
	}
}

// Name 返回处理器名称
func (h *RawJsonDataHandlerV2) Name() string {
	return "原始JSON数据处理器V2"
}

// Handle 处理任务（使用公共ProductFetcher）
func (h *RawJsonDataHandlerV2) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始获取原始JSON数据")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品id为空")
	}

	// 使用公共ProductFetcher获取产品数据
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(req)
	if err != nil {
		// 检查是否为产品不存在错误（不可重试）
		var productNotFoundErr *model.ProductNotFoundError
		if errors.As(err, &productNotFoundErr) {
			h.logger.Errorf("❌ 产品不存在或无法访问，标记为不可重试: %v", err)
			return types.NewNonRetryableError(
				fmt.Sprintf("产品不存在或无法访问: %s", productNotFoundErr.Message),
				err,
			)
		}
		return fmt.Errorf("获取产品数据失败: %w", err)
	}

	// 将Amazon产品数据存储到上下文中
	ctx.AmazonProduct = amazonProduct

	return nil
}

// Shutdown 关闭处理器，释放资源（现在由共享的Amazon处理器管理）
func (h *RawJsonDataHandlerV2) Shutdown() {
	// Amazon处理器由外部管理，不需要在这里关闭
	h.logger.Debug("RawJsonDataHandlerV2 关闭（Amazon处理器由外部管理）")
}
