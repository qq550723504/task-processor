// Package operation 提供SHEIN平台调度器相关服务
package operation

import (
	"encoding/json"
	"fmt"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// fillProductLevelInventory 填充产品级别的库存信息
func (s *productSyncServiceImpl) fillProductLevelInventory(
	productData *managementapi.ProductDataDTO,
	inventoryInfo *product.InventoryInfo,
) {
	if inventoryInfo == nil || len(inventoryInfo.SkcInfo) == 0 {
		return
	}

	totalUsable := 0
	for _, skcInv := range inventoryInfo.SkcInfo {
		for _, skuInv := range skcInv.SkuInfo {
			for _, warehouse := range skuInv.InventoryInfo {
				totalUsable += warehouse.UsableInventory
			}
		}
	}

	productData.Stock = types.FlexibleString(fmt.Sprintf("%d", totalUsable))

	s.logger.WithFields(logrus.Fields{
		"spu_code":         inventoryInfo.SpuName,
		"usable_inventory": totalUsable,
	}).Debug("填充产品级别库存信息")
}

// fillProductLevelPrice 填充产品级别的价格信息
func (s *productSyncServiceImpl) fillProductLevelPrice(
	productData *managementapi.ProductDataDTO,
	priceMap map[string]*product.SkuPriceInfo,
	costMap map[string]*product.SkuCostInfo,
) {
	// 从价格信息中获取第一个SKU的价格作为产品级别价格
	for _, priceInfo := range priceMap {
		if len(priceInfo.PriceInfoList) > 0 {
			firstPrice := priceInfo.PriceInfoList[0]
			productData.OriginalPrice = types.FlexibleString(fmt.Sprintf("%.2f", firstPrice.ShopPrice))
			productData.SpecialPrice = types.FlexibleString(fmt.Sprintf("%.2f", firstPrice.SpecialPrice))
			productData.PriceCurrency = firstPrice.Currency
			return
		}
	}

	// 如果没有价格信息，尝试从成本价获取
	for _, costInfo := range costMap {
		productData.OriginalPrice = types.FlexibleString(costInfo.CostPriceInfo.CostPrice)
		productData.PriceCurrency = costInfo.CostPriceInfo.Currency
		return
	}
}

// enrichProductWithMappingBySku 通过SKU查询映射表并填充产品数据
func (s *productSyncServiceImpl) enrichProductWithMappingBySku(
	productData *managementapi.ProductDataDTO,
	sheinProduct *product.ProductListItem,
	_ int64, storeID int64,
	inventoryInfo *product.InventoryInfo,
	priceMap map[string]*product.SkuPriceInfo,
	costMap map[string]*product.SkuCostInfo,
) {
	if s.mappingClient == nil {
		s.logger.Debug("映射客户端未设置，跳过数据增强")
		return
	}

	// 构建SKU库存映射
	skuInventoryMap := s.buildSkuInventoryMap(inventoryInfo)

	foundMapping := false
	var firstAsin string
	var firstParentAsin string

	// 创建增强的SKC数据结构
	enrichedSkcList := make([]EnrichedSkcInfo, 0, len(sheinProduct.SkcInfoList))

	for _, skc := range sheinProduct.SkcInfoList {
		enrichedSkc := EnrichedSkcInfo{
			SkcName:               skc.SkcName,
			SkcCode:               skc.SkcCode,
			SaleName:              skc.SaleName,
			MainImageThumbnailURL: skc.MainImageThumbnailURL,
			SupplierCode:          skc.SupplierCode,
			BusinessModel:         skc.BusinessModel,
			IsSaleAttribute:       skc.IsSaleAttribute,
			SupplierID:            skc.SupplierID,
			SkuInfo:               make([]EnrichedSkuInfo, 0, len(skc.SkuInfo)),
			MallSellStatus:        skc.MallSellStatus,
			Abandoned:             skc.Abandoned,
			TagInfoList:           skc.TagInfoList,
			ShelfFailReason:       skc.ShelfFailReason,
			HasOriginalImage:      skc.HasOriginalImage,
		}

		for _, sku := range skc.SkuInfo {
			enrichedSku := s.buildEnrichedSkuInfo(sku, priceMap, costMap, skuInventoryMap, storeID)
			enrichedSkc.SkuInfo = append(enrichedSkc.SkuInfo, enrichedSku)

			if enrichedSku.MappingInfo != nil {
				if !foundMapping && enrichedSku.MappingInfo.ProductId != "" {
					firstAsin = enrichedSku.MappingInfo.ProductId
					if enrichedSku.MappingInfo.ParentProductId != nil {
						firstParentAsin = *enrichedSku.MappingInfo.ParentProductId
					}
					foundMapping = true
				}
			}
		}

		enrichedSkcList = append(enrichedSkcList, enrichedSkc)
	}

	// 使用第一个找到的ASIN填充产品级别的ProductID和Region
	if foundMapping {
		productData.ProductID = firstAsin
		if firstParentAsin != "" {
			productData.ParentProductID = firstParentAsin
		}

		s.fillProductRegion(productData, enrichedSkcList)
		s.updateAttributesWithMappings(productData, enrichedSkcList)
	}
}

// buildSkuInventoryMap 构建SKU库存映射
func (s *productSyncServiceImpl) buildSkuInventoryMap(inventoryInfo *product.InventoryInfo) map[string][]product.WarehouseInventory {
	skuInventoryMap := make(map[string][]product.WarehouseInventory)

	if inventoryInfo == nil {
		return skuInventoryMap
	}

	for _, skcInv := range inventoryInfo.SkcInfo {
		for _, skuInv := range skcInv.SkuInfo {
			skuInventoryMap[skuInv.SkuCode] = skuInv.InventoryInfo
		}
	}

	return skuInventoryMap
}

// buildEnrichedSkuInfo 构建增强的SKU信息
func (s *productSyncServiceImpl) buildEnrichedSkuInfo(
	sku product.SkuInfo,
	priceMap map[string]*product.SkuPriceInfo,
	costMap map[string]*product.SkuCostInfo,
	skuInventoryMap map[string][]product.WarehouseInventory,
	storeID int64,
) EnrichedSkuInfo {
	enrichedSku := EnrichedSkuInfo{
		SkuInfo: sku,
	}

	// 填充价格信息
	if skuPriceInfo, ok := priceMap[sku.SkuCode]; ok {
		enrichedSku.SaleNameInfo = skuPriceInfo.SaleNameInfo
		enrichedSku.PriceInfoList = skuPriceInfo.PriceInfoList
	}

	// 填充成本价信息
	if skuCostInfo, ok := costMap[sku.SkuCode]; ok {
		enrichedSku.SaleAttributeList = skuCostInfo.SaleAttributeList
		enrichedSku.CostPriceInfo = &skuCostInfo.CostPriceInfo
	}

	// 填充库存信息
	if warehouseList, ok := skuInventoryMap[sku.SkuCode]; ok {
		enrichedSku.InventoryInfo = warehouseList

		totalUsable := 0
		totalInventory := 0
		for _, warehouse := range warehouseList {
			totalUsable += warehouse.UsableInventory
			totalInventory += warehouse.InventoryQuantity
		}
		enrichedSku.UsableInventory = &totalUsable
		enrichedSku.InventoryQuantity = &totalInventory
	}

	// 查询映射关系
	mapping, err := s.mappingClient.GetProductImportMappingByPlatformProductId(&managementapi.ProductImportMappingGetReqDTO{
		PlatformProductId: sku.SkuCode,
	})

	if err != nil || mapping == nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"sku_code": sku.SkuCode,
			"store_id": storeID,
		}).Warn("查询SKU映射关系失败")

		// 尝试自动修复映射关系
		// if s.repairService != nil {
		// 	s.logger.WithFields(logrus.Fields{
		// 		"sku_code": sku.SkuCode,
		// 		"store_id": storeID,
		// 	}).Info("尝试自动修复SKU映射关系")

		// 	repairResult, repairErr := s.repairService.AutoRepairMapping(
		// 		context.Background(),
		// 		sku.SkuCode,
		// 		storeID,
		// 		fmt.Sprintf("查询映射关系失败: %v", err),
		// 	)

		// 	if repairErr != nil {
		// 		s.logger.WithError(repairErr).WithFields(logrus.Fields{
		// 			"sku_code": sku.SkuCode,
		// 			"store_id": storeID,
		// 		}).Warn("自动修复SKU映射关系失败")
		// 	} else if repairResult != nil && repairResult.Success {
		// 		s.logger.WithFields(logrus.Fields{
		// 			"sku_code": sku.SkuCode,
		// 			"store_id": storeID,
		// 		}).Info("自动修复SKU映射关系成功")
		// 		enrichedSku.MappingInfo = repairResult.MappingInfo
		// 	} else {
		// 		s.logger.WithFields(logrus.Fields{
		// 			"sku_code": sku.SkuCode,
		// 			"store_id": storeID,
		// 			"error":    repairResult.Error,
		// 		}).Warn("自动修复SKU映射关系未成功")
		// 	}
		// }
	} else {
		// 查询成功，mapping不为nil
		enrichedSku.MappingInfo = mapping
	}

	return enrichedSku
}

// fillProductRegion 填充产品区域信息
func (s *productSyncServiceImpl) fillProductRegion(productData *managementapi.ProductDataDTO, enrichedSkcList []EnrichedSkcInfo) {
	for _, enrichedSkc := range enrichedSkcList {
		for _, enrichedSku := range enrichedSkc.SkuInfo {
			if enrichedSku.MappingInfo != nil && enrichedSku.MappingInfo.Region != "" {
				productData.Region = enrichedSku.MappingInfo.Region
				return
			}
		}
	}
}

// updateAttributesWithMappings 更新Attributes，包含SKU级别的映射信息
func (s *productSyncServiceImpl) updateAttributesWithMappings(productData *managementapi.ProductDataDTO, enrichedSkcList []EnrichedSkcInfo) {
	if attributesJSON, err := json.Marshal(enrichedSkcList); err == nil {
		productData.Attributes = string(attributesJSON)
		s.logger.Debug("已更新Attributes，包含SKU级别的完整信息")
	} else {
		s.logger.WithError(err).Warn("序列化增强的Attributes失败")
	}
}
