// Package handler 提供Amazon库存处理器
package handler

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/api"
	"task-processor/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// InventoryHandler 库存处理器
type InventoryHandler struct {
	*BaseHandler
}

// NewInventoryHandler 创建库存处理器
func NewInventoryHandler() *InventoryHandler {
	return &InventoryHandler{
		BaseHandler: NewBaseHandler("库存处理器"),
	}
}

// Execute 处理逻辑
func (h *InventoryHandler) Execute(services *model.Services, data map[string]any) error {
	h.logger.Info("开始设置库存")

	// 验证服务
	if err := h.ValidateServices(services); err != nil {
		return err
	}

	// 获取API客户端
	apiClient, err := h.GetAPIClient(services)
	if err != nil {
		return err
	}

	// 获取SKU
	sku, err := h.GetRequiredString(data, "listing_sku")
	if err != nil {
		return fmt.Errorf("获取SKU失败: %w", err)
	}

	// 获取库存数量
	quantity := h.getInventoryQuantity(data)

	// 构建库存请求
	req := &api.InventoryRequest{
		SKU:      sku,
		Quantity: quantity,
	}

	// 获取上下文
	ctxValue, exists := data["context"]
	if !exists {
		return fmt.Errorf("上下文不存在")
	}

	ctx, ok := ctxValue.(context.Context)
	if !ok {
		return fmt.Errorf("上下文类型错误")
	}

	// 更新库存
	resp, err := apiClient.UpdateInventory(ctx, req)
	if err != nil {
		return fmt.Errorf("更新库存失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"sku":      resp.SKU,
		"quantity": resp.Quantity,
	}).Info("库存设置成功")

	// 保存结果
	h.SetResult(data, "inventory_updated", true)
	h.SetResult(data, "inventory_quantity", resp.Quantity)

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

	// 2. 从映射属性中获取
	if mappedAttrs, exists := data["mapped_attributes"]; exists {
		if attrs, ok := mappedAttrs.(map[string]any); ok {
			if qty, ok := attrs["quantity"].(int); ok && qty > 0 {
				return qty
			}
		}
	}

	// 3. 使用默认值
	return 100
}
