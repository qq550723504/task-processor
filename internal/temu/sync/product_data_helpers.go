// package sync 提供TEMU平台调度器相关服务
package sync

import (
	managementapi "task-processor/internal/infra/clients/management/api"
)

// buildProductDataItem 将 ProductDataDTO 转换为 ProductDataItemDTO
func buildProductDataItem(prod *managementapi.ProductDataDTO) managementapi.ProductDataItemDTO {
	return managementapi.ProductDataItemDTO{
		PlatformProductID:  prod.PlatformProductID,
		ProductName:        prod.Title,
		ProductSku:         prod.ProductID,
		ProductPrice:       prod.OriginalPrice,
		ProductStock:       prod.Stock,
		ProductCategory:    prod.Category,
		ProductImage:       prod.MainImageURL,
		ProductDescription: prod.Description,
		ShelfStatus:        &prod.ShelfStatus,
		PublishTime:        prod.PublishTime,
		ShelfTime:          prod.ShelfTime,
		Brand:              prod.Brand,
		CategoryID:         &prod.CategoryID,
		SpecialPrice:       prod.SpecialPrice,
		PriceCurrency:      prod.PriceCurrency,
		ImageUrls:          prod.ImageURLs,
		Attributes:         prod.Attributes,
		PlatformStatus:     prod.PlatformStatus,
		PlatformData:       prod.PlatformData,
		ParentProductID:    prod.ParentProductID,
		CreateTime:         prod.CreateTime,
		UpdateTime:         prod.UpdateTime,
	}
}

// buildBatchSaveReq 构建批量保存请求
func buildBatchSaveReq(prod *managementapi.ProductDataDTO, items []managementapi.ProductDataItemDTO) *managementapi.ProductDataBatchSaveReqDTO {
	return &managementapi.ProductDataBatchSaveReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		Region:   prod.Region,
		StoreID:  prod.StoreID,
		Products: items,
	}
}
