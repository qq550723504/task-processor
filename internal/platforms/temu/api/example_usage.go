// Package api 提供使用示例
package api

import (
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// ExampleUsage 展示如何使用重构后的API
func ExampleUsage() {
	// 创建管理客户端
	managementClient := &management.ClientManager{} // 实际使用时需要正确初始化

	// 创建API客户端
	apiClient := NewAPIClient(12345, 67890, managementClient)

	// 创建各种API服务
	logger := logrus.WithField("example", "usage")

	productAPI := NewProductAPI(apiClient, logger)
	pricingAPI := NewPricingAPI(apiClient, logger)
	listingAPI := NewListingAPI(apiClient, logger)
	offlineAPI := NewOfflineAPI(apiClient, logger)
	imageUploadAPI := NewImageUploadAPI(apiClient, logger)

	// 使用产品API
	_, _ = productAPI.ListProducts(1, 20)

	// 使用定价API
	_, _ = pricingAPI.GetPendingPriceList(1, 20)

	// 使用上架API
	_, _ = listingAPI.RelistProduct("goods123", []string{"sku1", "sku2"})

	// 使用下架产品API
	_, _ = offlineAPI.GetOfflineProducts(1, 20)

	// 使用图片上传API
	_, _ = imageUploadAPI.UploadImage("https://example.com/image.jpg")
}
