// Package handlers 提供TEMU平台的原始JSON数据处理功能
package handlers

import (
	"errors"
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandlerV2 原始JSON数据处理器V2（使用工厂模式选择获取器）
type RawJsonDataHandlerV2 struct {
	logger  *logrus.Entry
	fetcher product.ProductFetcherInterface
}

// NewRawJsonDataHandlerV2 创建新的原始JSON数据处理器V2（支持分布式获取器）
func NewRawJsonDataHandlerV2(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor *amazon.AmazonProcessor,
) *RawJsonDataHandlerV2 {
	logger := logrus.WithField("handler", "RawJsonDataHandlerV2")

	// 使用工厂模式创建获取器
	factory := product.NewFetcherFactory()

	// 根据配置创建获取器
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		// 降级到本地获取器
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	logger.Infof("✅ 产品获取器创建成功，类型: %s", factory.GetRecommendedFetcher(cfg))

	return &RawJsonDataHandlerV2{
		logger:  logger,
		fetcher: fetcher,
	}
}

// Name 返回处理器名称
func (h *RawJsonDataHandlerV2) Name() string {
	return "原始JSON数据处理器V2"
}

// Handle 处理任务（使用公共ProductFetcher）
func (h *RawJsonDataHandlerV2) Handle(ctx pipeline.TaskContext) error {
	h.logger.Info("开始获取原始JSON数据")

	// 检查任务上下文中的必要数据
	task := ctx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if task.ProductID == "" {
		return fmt.Errorf("产品id为空")
	}

	// 使用公共ProductFetcher获取产品数据
	req := &product.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.Platform,
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    task.Creator,
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
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		amazonCtx.SetAmazonProduct(amazonProduct)
	}
	return nil
}

// Shutdown 关闭处理器，释放资源
func (h *RawJsonDataHandlerV2) Shutdown() {
	h.logger.Debug("RawJsonDataHandlerV2 关闭")
}
