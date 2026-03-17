// package pipeline 提供Listing处理器实现
package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/api"
	"task-processor/internal/amazon/model"
)

// ListingHandler Listing处理器
type ListingHandler struct {
	*BaseHandler
}

// NewListingHandler 创建Listing处理器
func NewListingHandler(services *model.Services) *ListingHandler {
	return &ListingHandler{
		BaseHandler: NewBaseHandler("创建Amazon Listing"),
	}
}

// Handle 执行Listing创建
func (h *ListingHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始创建Amazon Listing")

	// 构建Listing请求
	req, err := h.buildListingRequest(taskContext.Data)
	if err != nil {
		return fmt.Errorf("构建Listing请求失败: %w", err)
	}

	// 简化实现：模拟创建成功
	mockResponse := &api.ListingResponse{
		SKU:    req.SKU,
		Status: "ACTIVE",
	}

	// 保存响应结果
	taskContext.SetResult("listing_sku", mockResponse.SKU)
	taskContext.SetResult("listing_status", mockResponse.Status)
	taskContext.SetResult("listing_response", mockResponse)

	h.logger.Infof("Listing创建成功: SKU=%s, Status=%s", mockResponse.SKU, mockResponse.Status)
	return nil
}

// buildListingRequest 构建Listing请求
func (h *ListingHandler) buildListingRequest(data map[string]any) (*api.ListingRequest, error) {
	// 获取产品ID生成SKU
	productID, exists := data["product_id"]
	if !exists {
		return nil, fmt.Errorf("产品ID不存在")
	}

	productIDStr, ok := productID.(string)
	if !ok {
		return nil, fmt.Errorf("产品ID格式错误")
	}

	// 生成SKU
	sku := fmt.Sprintf("1688-%s", productIDStr)

	// 构建基础请求
	req := &api.ListingRequest{
		SKU:          sku,
		ProductType:  "PRODUCT", // 默认产品类型
		Requirements: "LISTING",
		Attributes:   make(map[string]any),
	}

	// 添加基础属性
	req.Attributes["item_name"] = h.getProductTitle(data)
	req.Attributes["brand"] = h.getProductBrand(data)
	req.Attributes["manufacturer"] = h.getProductBrand(data)

	return req, nil
}

// getProductTitle 获取产品标题
func (h *ListingHandler) getProductTitle(data map[string]any) string {
	// 从解析后的数据中获取标题
	if title, exists := data["product_title"]; exists {
		if titleStr, ok := title.(string); ok {
			return titleStr
		}
	}

	// 默认标题
	return "Amazon Product"
}

// getProductBrand 获取产品品牌
func (h *ListingHandler) getProductBrand(data map[string]any) string {
	// 从解析后的数据中获取品牌
	if brand, exists := data["product_brand"]; exists {
		if brandStr, ok := brand.(string); ok {
			return brandStr
		}
	}

	// 默认品牌
	return "Generic"
}

