// Package handler 提供Listing处理器实现
package handler

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/api"
	"task-processor/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// ListingHandler Listing处理器
type ListingHandler struct {
	logger *logrus.Entry
}

// NewListingHandler 创建Listing处理器
func NewListingHandler() *ListingHandler {
	return &ListingHandler{
		logger: logrus.WithField("handler", "ListingHandler"),
	}
}

// Name 返回处理器名称
func (h *ListingHandler) Name() string {
	return "创建Amazon Listing"
}

// Execute 执行Listing创建
func (h *ListingHandler) Execute(services *model.Services, data map[string]interface{}) error {
	h.logger.Info("开始创建Amazon Listing")

	// 检查必要的服务
	if services.APIClient == nil {
		return fmt.Errorf("Amazon API客户端未初始化")
	}

	apiClient := services.APIClient

	// 构建Listing请求
	req, err := h.buildListingRequest(data)
	if err != nil {
		return fmt.Errorf("构建Listing请求失败: %w", err)
	}

	// 调用Amazon API创建Listing
	ctx := context.Background()
	resp, err := apiClient.CreateListing(ctx, req)
	if err != nil {
		return fmt.Errorf("创建Listing失败: %w", err)
	}

	// 保存响应结果
	data["listing_sku"] = resp.SKU
	data["listing_status"] = resp.Status
	data["listing_response"] = resp

	h.logger.Infof("Listing创建成功: SKU=%s, Status=%s", resp.SKU, resp.Status)
	return nil
}

// buildListingRequest 构建Listing请求
func (h *ListingHandler) buildListingRequest(data map[string]interface{}) (*api.ListingRequest, error) {
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
		Attributes:   make(map[string]interface{}),
	}

	// 添加基础属性
	req.Attributes["item_name"] = h.getProductTitle(data)
	req.Attributes["brand"] = h.getProductBrand(data)
	req.Attributes["manufacturer"] = h.getProductBrand(data)

	return req, nil
}

// getProductTitle 获取产品标题
func (h *ListingHandler) getProductTitle(data map[string]interface{}) string {
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
func (h *ListingHandler) getProductBrand(data map[string]interface{}) string {
	// 从解析后的数据中获取品牌
	if brand, exists := data["product_brand"]; exists {
		if brandStr, ok := brand.(string); ok {
			return brandStr
		}
	}

	// 默认品牌
	return "Generic"
}
