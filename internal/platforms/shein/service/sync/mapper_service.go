// Package sync 提供SHEIN平台产品数据映射功能
package sync

import (
	"fmt"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/model"
)

// MapToProductData 将SHEIN产品响应映射为后端产品数据
func MapToProductData(sheinProduct *model.SheinProductResponse, storeID int64) (*api.ProductDataDTO, error) {
	if sheinProduct == nil {
		return nil, fmt.Errorf("sheinProduct 不能为空")
	}

	// TODO: 实现完整的产品数据映射逻辑
	productData := &api.ProductDataDTO{
		StoreID:           storeID,
		Platform:          "SHEIN",
		PlatformProductID: sheinProduct.SpuCode,
		Title:             sheinProduct.ProductNameEn,
		CategoryID:        sheinProduct.CategoryID,
		Brand:             sheinProduct.BrandName,
		ShelfStatus:       MapShelfStatus(sheinProduct.ShelfStatus),
	}

	return productData, nil
}

// MapShelfStatus 映射上架状态
func MapShelfStatus(status string) int {
	switch status {
	case "ON_SHELF":
		return 2 // 已上架
	case "OFF_SHELF":
		return 3 // 已下架
	default:
		return 0 // 待上架
	}
}
