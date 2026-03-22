// Package product 提供TEMU平台的原始JSON数据处理功能
package product

import (
	"errors"
	"fmt"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	domainProduct "task-processor/internal/product"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// RawJsonDataHandlerV2 原始JSON数据处理器V2（使用工厂模式选择获取器）
type RawJsonDataHandlerV2 struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcher
}

// NewRawJsonDataHandlerV2 创建新的原始JSON数据处理器V2（支持分布式获取器）
func NewRawJsonDataHandlerV2(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor domainProduct.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *RawJsonDataHandlerV2 {
	logger := logger.GetGlobalLogger("RawJsonDataHandlerV2")

	// 使用工厂模式创建获取器
	factory := appProduct.NewFetcherFactory()

	// 根据配置创建获取器
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		// 降级到本地获取器
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
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
	req := &domainProduct.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.SourcePlatform,
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(ctx.GetContext(), req)
	if err != nil {
		// 检查是否为产品不存在错误（不可重试）
		var productNotFoundErr *model.ProductNotFoundError
		if errors.As(err, &productNotFoundErr) {
			h.logger.Errorf("❌ 产品不存在或无法访问，标记为不可重试: %v", err)
			return fmt.Errorf("NONRETRYABLE: 产品不存在或无法访问: %s: %w", productNotFoundErr.Message, err)
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
