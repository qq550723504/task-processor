// Package product 提供TEMU平台的原始JSON数据处理功能
package product

import (
	"errors"
	"fmt"
	appProduct "task-processor/internal/crawler/fetcher"
	"task-processor/internal/marketplace/sourceproduct"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandlerV2 原始JSON数据处理器V2（使用工厂模式选择获取器）
type RawJsonDataHandlerV2 struct {
	logger *logrus.Entry
	reader appProduct.ProductReader
}

// NewRawJsonDataHandlerV2 创建新的原始JSON数据处理器V2（支持分布式获取器）
func NewRawJsonDataHandlerV2(
	reader appProduct.ProductReader,
	stats appProduct.ProductFetcherStats,
) *RawJsonDataHandlerV2 {
	logger := logger.GetGlobalLogger("RawJsonDataHandlerV2")

	// 打印实际使用的获取器类型（通过 GetStats 判断）
	if stats != nil {
		if values := stats.GetStats(); values != nil {
			logger.Infof("✅ 产品获取器创建成功，实际类型: %v", values["type"])
		}
	}

	return &RawJsonDataHandlerV2{
		logger: logger,
		reader: reader,
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
	req := &sourceproduct.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.GetSourcePlatformOrDefault(),
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    task.Creator,
	}

	amazonProduct, err := h.reader.FetchProduct(ctx.GetContext(), req)
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
