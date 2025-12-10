package handlers

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// InventorySetupHandler 库存设置处理器
type InventorySetupHandler struct {
	apiClient *api.Client
}

// NewInventorySetupHandler 创建库存设置处理器
func NewInventorySetupHandler(apiClient *api.Client) *InventorySetupHandler {
	return &InventorySetupHandler{
		apiClient: apiClient,
	}
}

// Name 返回处理器名称
func (h *InventorySetupHandler) Name() string {
	return "设置产品库存"
}

// Handle 处理逻辑
func (h *InventorySetupHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[InventorySetup] 开始设置产品库存")

	// 1. 获取SKU
	sku, exists := ctx.GetData("listing_sku")
	if !exists {
		return fmt.Errorf("SKU不存在")
	}

	skuStr, ok := sku.(string)
	if !ok {
		return fmt.Errorf("SKU格式错误")
	}

	// 2. 从原始数据中获取库存数量
	quantity := h.extractQuantity(ctx)

	// 3. 构建库存请求
	req := &api.InventoryRequest{
		SKU:      skuStr,
		Quantity: quantity,
	}

	// 4. 调用API设置库存
	apiCtx := context.Background()
	resp, err := h.apiClient.UpdateInventory(apiCtx, req)
	if err != nil {
		return fmt.Errorf("设置库存失败: %w", err)
	}

	// 5. 保存结果
	ctx.SetData("inventory_quantity", resp.Quantity)
	ctx.SetData("inventory_status", resp.Status)

	logrus.Infof("[InventorySetup] 库存设置完成: SKU=%s, Quantity=%d", skuStr, resp.Quantity)
	return nil
}

// extractQuantity 从产品数据中提取库存数量
func (h *InventorySetupHandler) extractQuantity(ctx *amazon.TaskContext) int {
	// 从原始产品数据中获取
	rawData, exists := ctx.GetData("raw_product_data")
	if !exists {
		logrus.Warn("[InventorySetup] 未找到原始产品数据，使用默认库存")
		return 100 // 默认库存
	}

	productData, ok := rawData.(map[string]interface{})
	if !ok {
		logrus.Warn("[InventorySetup] 产品数据格式错误，使用默认库存")
		return 100
	}

	// 尝试从不同字段获取库存
	quantityFields := []string{
		"quantity",
		"stock",
		"inventory",
		"availableQuantity",
	}

	for _, field := range quantityFields {
		if val, ok := productData[field]; ok {
			switch v := val.(type) {
			case int:
				if v > 0 {
					return v
				}
			case float64:
				if v > 0 {
					return int(v)
				}
			case string:
				var qty int
				if _, err := fmt.Sscanf(v, "%d", &qty); err == nil && qty > 0 {
					return qty
				}
			}
		}
	}

	// 默认库存
	logrus.Info("[InventorySetup] 未找到库存字段，使用默认值100")
	return 100
}
