// package pipeline 提供Amazon库存处理器
package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/api"
	"task-processor/internal/amazon/model"
)

// InventoryHandler 库存处理器
type InventoryHandler struct {
	*BaseHandler
}

// NewInventoryHandler 创建库存处理器
func NewInventoryHandler(services *model.Services) *InventoryHandler {
	return &InventoryHandler{
		BaseHandler: NewBaseHandler("库存处理器"),
	}
}

// Handle 处理逻辑
func (h *InventoryHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始设置库存")

	// 获取SKU（从Results中获取）
	skuValue, exists := taskContext.GetResult("listing_sku")
	if !exists {
		return fmt.Errorf("SKU不存在")
	}

	sku, ok := skuValue.(string)
	if !ok {
		return fmt.Errorf("SKU格式错误")
	}

	// 获取库存数量
	quantity := h.getInventoryQuantity(taskContext.Data)

	// 模拟库存更新成功
	mockResponse := &api.InventoryResponse{
		SKU:      sku,
		Quantity: quantity,
		Status:   "ACTIVE",
	}

	h.logger.WithFields(map[string]any{
		"sku":      mockResponse.SKU,
		"quantity": mockResponse.Quantity,
	}).Info("库存设置成功")

	// 保存结果
	taskContext.SetResult("inventory_updated", true)
	taskContext.SetResult("inventory_quantity", mockResponse.Quantity)

	return nil
}

// getInventoryQuantity 获取库存数量
func (h *InventoryHandler) getInventoryQuantity(data map[string]any) int {
	// 1. 从产品数据中获取
	if rawData, exists := data["raw_product_data"]; exists {
		if productData, ok := rawData.(map[string]any); ok {
			if qty, ok := productData["quantity"].(int); ok && qty > 0 {
				return qty
			}
			if qty, ok := productData["stock"].(int); ok && qty > 0 {
				return qty
			}
		}
	}

	// 2. 使用默认值
	return 100
}

