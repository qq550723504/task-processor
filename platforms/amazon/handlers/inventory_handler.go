package handlers

import (
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// InventoryHandler 库存处理器
type InventoryHandler struct {
	apiClient *api.Client
}

// NewInventoryHandler 创建库存处理器
func NewInventoryHandler(apiClient *api.Client) *InventoryHandler {
	return &InventoryHandler{
		apiClient: apiClient,
	}
}

// Name 返回处理器名称
func (h *InventoryHandler) Name() string {
	return "设置库存"
}

// Handle 处理逻辑
func (h *InventoryHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("开始设置库存")

	// 获取SKU
	sku := ctx.Task.ProductID

	// 构建库存请求
	req := &api.InventoryRequest{
		SKU:      sku,
		Quantity: h.getDefaultQuantity(ctx),
	}

	// 更新库存
	resp, err := h.apiClient.UpdateInventory(ctx.Context, req)
	if err != nil {
		return fmt.Errorf("更新库存失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"sku":      resp.SKU,
		"quantity": resp.Quantity,
	}).Info("库存设置成功")

	return nil
}

// getDefaultQuantity 获取默认库存数量
func (h *InventoryHandler) getDefaultQuantity(ctx *amazon.TaskContext) int {
	// TODO: 从配置或产品数据中获取库存数量
	return 100
}
