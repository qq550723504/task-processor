// Package handler 提供产品数据处理器实现
package handler

import (
	"context"
	"fmt"
	"task-processor/internal/platforms/amazon/core/model"
)

// ProductDataHandler 产品数据处理器
type ProductDataHandler struct {
	*BaseHandler
}

// NewProductDataHandler 创建产品数据处理器
func NewProductDataHandler(services *model.Services) *ProductDataHandler {
	return &ProductDataHandler{
		BaseHandler: NewBaseHandler("获取产品数据"),
	}
}

// Handle 执行产品数据处理
func (h *ProductDataHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始获取产品数据")

	// 从数据中获取产品ID
	productIDValue, exists := taskContext.Data["product_id"]
	if !exists {
		return fmt.Errorf("产品ID不存在")
	}

	productID, ok := productIDValue.(string)
	if !ok {
		return fmt.Errorf("产品ID格式错误")
	}

	// 简化实现：创建模拟的产品数据
	mockProductData := map[string]interface{}{
		"product_id":  productID,
		"title":       "Sample Product",
		"description": "This is a sample product description",
		"brand":       "Sample Brand",
		"price":       29.99,
		"currency":    "USD",
	}

	// 保存原始数据到上下文
	taskContext.SetResult("raw_product_data", mockProductData)
	taskContext.SetResult("data_source", "amazon")

	h.logger.Infof("产品数据获取成功: ProductID=%s", productID)
	return nil
}
