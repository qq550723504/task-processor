// Package handler 提供Amazon变体产品处理器
package handler

import (
	"context"
	"fmt"
	"task-processor/internal/platforms/amazon/core/model"
	amazonutil "task-processor/internal/platforms/amazon/utils"
)

// VariantHandler 变体产品处理器
type VariantHandler struct {
	*BaseHandler
	extractor *amazonutil.VariantExtractor
}

// NewVariantHandler 创建变体处理器
func NewVariantHandler(services *model.Services) *VariantHandler {
	return &VariantHandler{
		BaseHandler: NewBaseHandler("变体处理器"),
		extractor:   amazonutil.NewVariantExtractor(),
	}
}

// Handle 处理逻辑
func (h *VariantHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始处理变体产品")

	// 1. 获取原始产品数据（从Results中获取）
	rawData, exists := taskContext.GetResult("raw_product_data")
	if !exists {
		return fmt.Errorf("产品数据不存在")
	}

	productData, ok := rawData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 2. 简化变体处理：检查是否有变体相关字段
	hasVariants := false
	if _, exists := productData["variants"]; exists {
		hasVariants = true
	}
	if _, exists := productData["colors"]; exists {
		hasVariants = true
	}
	if _, exists := productData["sizes"]; exists {
		hasVariants = true
	}

	// 3. 设置变体处理结果
	if !hasVariants {
		h.logger.Info("这是单品，跳过变体处理")
		taskContext.SetResult("is_variant_product", false)
		return nil
	}

	h.logger.Info("检测到变体产品")
	taskContext.SetResult("is_variant_product", true)
	taskContext.SetResult("variation_theme", "ColorSize")
	taskContext.SetResult("variant_children_count", 2)

	h.logger.Info("变体处理完成")
	return nil
}
