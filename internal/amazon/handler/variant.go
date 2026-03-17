// Package handler 提供Amazon变体产品处理器
package handler

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/model"
)

// VariantHandler 变体产品处理器
type VariantHandler struct {
	*BaseHandler
}

// NewVariantHandler 创建变体处理器
func NewVariantHandler(services *model.Services) *VariantHandler {
	return &VariantHandler{
		BaseHandler: NewBaseHandler("变体处理器"),
	}
}

// Handle 处理逻辑
func (h *VariantHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始处理变体产品")

	rawData, exists := taskContext.GetResult("raw_product_data")
	if !exists {
		return fmt.Errorf("产品数据不存在")
	}

	productData, ok := rawData.(map[string]any)
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	hasVariants := false
	for _, key := range []string{"variants", "colors", "sizes"} {
		if _, exists := productData[key]; exists {
			hasVariants = true
			break
		}
	}

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
