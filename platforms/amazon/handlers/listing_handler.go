package handlers

import (
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// ListingHandler 创建listing处理器
type ListingHandler struct {
	apiClient *api.Client
}

// NewListingHandler 创建listing处理器
func NewListingHandler(apiClient *api.Client) *ListingHandler {
	return &ListingHandler{
		apiClient: apiClient,
	}
}

// Name 返回处理器名称
func (h *ListingHandler) Name() string {
	return "创建Amazon Listing"
}

// Handle 处理逻辑
func (h *ListingHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("开始创建Amazon listing")

	// 构建listing请求
	req := h.buildListingRequest(ctx)

	// 调用Amazon SP-API创建listing
	resp, err := h.apiClient.CreateListing(ctx.Context, req)
	if err != nil {
		return fmt.Errorf("创建listing失败: %w", err)
	}

	// 保存响应到上下文
	ctx.SetData("listing_response", resp)

	logrus.WithFields(logrus.Fields{
		"sku":    resp.SKU,
		"status": resp.Status,
	}).Info("Listing创建成功")

	return nil
}

// buildListingRequest 构建listing请求
func (h *ListingHandler) buildListingRequest(ctx *amazon.TaskContext) *api.ListingRequest {
	// TODO: 从上下文中提取产品数据并构建请求
	return &api.ListingRequest{
		SKU:         ctx.Task.ProductID,
		ProductType: "PRODUCT",
		Attributes:  make(map[string]interface{}),
	}
}
