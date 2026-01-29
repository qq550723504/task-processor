// Package scheduler 提供TEMU平台调度器相关服务
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	temuapi "task-processor/internal/platforms/temu/api"

	"github.com/sirupsen/logrus"
)

// delistProductViaTEMUAPI 通过TEMU API下架产品
func (s *inventorySyncServiceImpl) delistProductViaTEMUAPI(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuInfo *TemuSkuInfo,
) error {
	goodsID := prod.PlatformProductID

	// 解析 Attributes 获取 SKU 信息
	var skuList []TemuSkuInfo
	if err := json.Unmarshal([]byte(prod.Attributes), &skuList); err != nil {
		return fmt.Errorf("解析产品attributes失败: %w", err)
	}

	// 收集需要下架的SKU ID列表
	var skuIDs []string
	for _, sku := range skuList {
		if s.getStringValue(sku.MappingInfo.Sku) != "" {
			skuIDs = append(skuIDs, s.getStringValue(sku.MappingInfo.Sku))
		}
	}

	if len(skuIDs) == 0 {
		return fmt.Errorf("未找到有效的SKU ID")
	}

	// 创建库存服务实例
	inventoryService := temuapi.NewInventoryService(s.temuAPIClient, s.logger)

	// 调用TEMU API下架产品
	if _, err := inventoryService.OfflineProduct(goodsID, skuIDs); err != nil {
		return fmt.Errorf("调用TEMU API下架产品失败: %w", err)
	}

	// 更新管理系统中的产品状态
	prod.ShelfStatus = managementapi.ShelfStatusOffShelf
	platformStatus := map[string]string{
		"shelf_status": "OFF_SHELF",
	}
	platformStatusJSON, _ := json.Marshal(platformStatus)
	prod.PlatformStatus = string(platformStatusJSON)

	// 构建批量更新请求
	productItem := managementapi.ProductDataItemDTO{
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

	batchReq := &managementapi.ProductDataBatchSaveReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		Region:   prod.Region,
		StoreID:  prod.StoreID,
		Products: []managementapi.ProductDataItemDTO{productItem},
	}

	productDataAPI := s.managementClient.GetProductDataClient(prod.StoreID)
	if _, err := productDataAPI.BatchCreateOrUpdate(batchReq); err != nil {
		s.logger.WithError(err).Warn("更新管理系统中的产品状态失败")
	}

	return nil
}

// updateProductStockViaTEMUAPI 通过TEMU API更新产品库存
func (s *inventorySyncServiceImpl) updateProductStockViaTEMUAPI(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuInfo *TemuSkuInfo,
	targetStock int,
) error {
	goodsID := prod.PlatformProductID

	// 创建库存服务实例
	inventoryService := temuapi.NewInventoryService(s.temuAPIClient, s.logger)

	// 构建库存变更信息
	skuStockChanges := []temuapi.SkuStockChange{
		{
			SkuID:                 skuInfo.SkuCode,
			CurrentShippingMode:   1,
			CurrentStockAvailable: skuInfo.UsableInventory,
			StockDiff:             targetStock - skuInfo.UsableInventory,
		},
	}

	// 调用TEMU API更新库存
	if _, err := inventoryService.EditStock(goodsID, skuStockChanges); err != nil {
		return fmt.Errorf("调用TEMU API更新库存失败: %w", err)
	}

	return nil
}

// relistProductViaTEMUAPI 通过TEMU API重新上架产品
func (s *inventorySyncServiceImpl) relistProductViaTEMUAPI(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuInfo *TemuSkuInfo,
) error {
	goodsID := prod.PlatformProductID
	platformSKU := s.getStringValue(skuInfo.MappingInfo.Sku)

	s.logger.WithFields(logrus.Fields{
		"product_id": prod.ProductID,
		"goods_id":   goodsID,
		"sku":        platformSKU,
	}).Info("开始通过TEMU API重新上架产品")

	// 解析 Attributes 获取 SKU 信息
	var skuList []TemuSkuInfo
	if err := json.Unmarshal([]byte(prod.Attributes), &skuList); err != nil {
		return fmt.Errorf("解析产品attributes失败: %w", err)
	}

	// 收集需要上架的SKU ID列表
	var skuIDs []string
	for _, sku := range skuList {
		if s.getStringValue(sku.MappingInfo.Sku) != "" {
			skuIDs = append(skuIDs, s.getStringValue(sku.MappingInfo.Sku))
		}
	}

	if len(skuIDs) == 0 {
		return fmt.Errorf("未找到有效的SKU ID")
	}

	// 创建库存服务实例
	inventoryService := temuapi.NewInventoryService(s.temuAPIClient, s.logger)

	// 调用TEMU API上架产品
	if _, err := inventoryService.OnlineProduct(goodsID, skuIDs); err != nil {
		return fmt.Errorf("调用TEMU API上架产品失败: %w", err)
	}

	// 更新管理系统中的产品状态
	prod.ShelfStatus = managementapi.ShelfStatusOnShelf
	platformStatus := map[string]string{
		"shelf_status": "ON_SHELF",
	}
	platformStatusJSON, _ := json.Marshal(platformStatus)
	prod.PlatformStatus = string(platformStatusJSON)

	// 构建批量更新请求
	productItem := managementapi.ProductDataItemDTO{
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

	batchReq := &managementapi.ProductDataBatchSaveReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		Region:   prod.Region,
		StoreID:  prod.StoreID,
		Products: []managementapi.ProductDataItemDTO{productItem},
	}

	productDataAPI := s.managementClient.GetProductDataClient(prod.StoreID)
	if _, err := productDataAPI.BatchCreateOrUpdate(batchReq); err != nil {
		s.logger.WithError(err).Warn("更新管理系统中的产品状态失败")
	}

	s.logger.WithFields(logrus.Fields{
		"product_id": prod.ProductID,
		"goods_id":   goodsID,
	}).Info("产品已成功通过TEMU API重新上架")

	return nil
}
