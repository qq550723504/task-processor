package handlers

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// ListingCreationHandler Listing创建处理器
type ListingCreationHandler struct {
	apiClient *api.Client
}

// NewListingCreationHandler 创建Listing创建处理器
func NewListingCreationHandler(apiClient *api.Client) *ListingCreationHandler {
	return &ListingCreationHandler{
		apiClient: apiClient,
	}
}

// Name 返回处理器名称
func (h *ListingCreationHandler) Name() string {
	return "创建Amazon Listing"
}

// Handle 处理逻辑
func (h *ListingCreationHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[ListingCreation] 开始创建Amazon Listing")

	// 1. 获取映射后的属性
	mappedAttrs, exists := ctx.GetData("mapped_attributes")
	if !exists {
		return fmt.Errorf("映射后的属性不存在")
	}

	attributes, ok := mappedAttrs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("属性数据格式错误")
	}

	// 2. 获取产品类型
	productType, _ := ctx.GetData("product_type")
	productTypeStr, ok := productType.(string)
	if !ok || productTypeStr == "" {
		productTypeStr = "PRODUCT" // 默认产品类型
	}

	// 3. 获取图片URL
	imageURLs, exists := ctx.GetData("image_urls")
	if exists {
		if urls, ok := imageURLs.([]string); ok && len(urls) > 0 {
			// 将图片URL添加到属性中
			attributes["main_product_image_locator"] = []map[string]string{
				{"media_location": urls[0]},
			}

			// 添加其他图片
			if len(urls) > 1 {
				otherImages := make([]map[string]string, 0, len(urls)-1)
				for i := 1; i < len(urls) && i < 9; i++ {
					otherImages = append(otherImages, map[string]string{
						"media_location": urls[i],
					})
				}
				attributes["other_product_image_locator"] = otherImages
			}
		}
	}

	// 4. 生成SKU
	sku := h.generateSKU(ctx)

	// 5. 构建Listing请求
	req := &api.ListingRequest{
		SKU:          sku,
		ProductType:  productTypeStr,
		Requirements: "LISTING",
		Attributes:   attributes,
	}

	// 6. 调用API创建Listing
	apiCtx := context.Background()
	resp, err := h.apiClient.CreateListing(apiCtx, req)
	if err != nil {
		return fmt.Errorf("创建Listing失败: %w", err)
	}

	// 7. 检查响应中的问题
	if len(resp.Issues) > 0 {
		logrus.Warn("[ListingCreation] Listing创建有问题:")
		for _, issue := range resp.Issues {
			logrus.Warnf("  - [%s] %s: %s", issue.Severity, issue.Code, issue.Message)
		}
	}

	// 8. 保存结果到上下文
	ctx.SetData("listing_sku", resp.SKU)
	ctx.SetData("listing_status", resp.Status)
	ctx.SetData("listing_response", resp)

	logrus.Infof("[ListingCreation] Listing创建完成: SKU=%s, Status=%s", resp.SKU, resp.Status)
	return nil
}

// generateSKU 生成SKU
func (h *ListingCreationHandler) generateSKU(ctx *amazon.TaskContext) string {
	// 从任务中获取产品ID
	if ctx.Task != nil && ctx.Task.ProductID != "" {
		// 使用产品ID作为SKU的一部分
		return fmt.Sprintf("1688-%s", ctx.Task.ProductID)
	}

	// 如果没有产品ID，使用时间戳
	return fmt.Sprintf("AMZN-%d", ctx.Task.CreateTime)
}
