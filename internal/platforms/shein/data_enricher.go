// Package shein 提供SHEIN平台数据增强功能
package shein

import (
	"encoding/json"
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/shein/api/product"
	"task-processor/internal/platforms/shein/modules"

	"github.com/sirupsen/logrus"
)

// DataEnricher SHEIN数据增强器
type DataEnricher struct {
	logger        *logrus.Entry
	mappingClient api.ProductImportMappingAPI
}

// NewDataEnricher 创建新的数据增强器
func NewDataEnricher() *DataEnricher {
	return &DataEnricher{
		logger: logrus.WithField("component", "DataEnricher"),
	}
}

// SetMappingClient 设置映射客户端
func (e *DataEnricher) SetMappingClient(mappingClient api.ProductImportMappingAPI) {
	e.mappingClient = mappingClient
}

// EnrichProductWithMappingBySku 通过 SKU 查询映射关系并填充 ASIN，同时填充 SKU 级别的完整价格/成本价/库存信息
func (e *DataEnricher) EnrichProductWithMappingBySku(productData *api.ProductDataDTO, sheinProduct *SheinProductResponse, tenantID, storeID int64, inventoryInfo *product.InventoryInfo, priceMap map[string]*product.SkuPriceInfo, costMap map[string]*product.SkuCostInfo) {
	if e.mappingClient == nil {
		return
	}

	// 构建 SKU Code 到库存信息的映射
	inventoryManager := NewInventoryManager()

	// 将产品库存信息转换为本地库存信息
	var localInventoryInfo *LocalInventoryInfo
	if inventoryInfo != nil {
		localInventoryInfo = &LocalInventoryInfo{
			SpuName:            inventoryInfo.SpuName,
			ProductNameCh:      inventoryInfo.ProductNameCh,
			MainImageThumbnail: inventoryInfo.MainImageThumbnail,
			IfFbmStore:         inventoryInfo.IfFbmStore,
			SkcInfo:            make([]LocalSkcInventory, len(inventoryInfo.SkcInfo)),
		}

		// 转换 SKC 信息
		for i, skcInfo := range inventoryInfo.SkcInfo {
			localInventoryInfo.SkcInfo[i] = LocalSkcInventory{
				SkcName:   skcInfo.SkcName,
				SortOrder: skcInfo.SortOrder,
				SkcCode:   skcInfo.SkcCode,
				SaleName:  skcInfo.SaleName,
				SkuInfo:   make([]LocalSkuInventory, len(skcInfo.SkuInfo)),
			}

			// 转换 SKU 信息
			for j, skuInfo := range skcInfo.SkuInfo {
				localInventoryInfo.SkcInfo[i].SkuInfo[j] = LocalSkuInventory{
					SkuCode:       skuInfo.SkuCode,
					SkuName:       skuInfo.SkuCode, // 使用 SkuCode 作为 SkuName
					InventoryInfo: make([]LocalWarehouseInventory, len(skuInfo.InventoryInfo)),
				}

				// 转换仓库库存信息
				for k, warehouseInfo := range skuInfo.InventoryInfo {
					localInventoryInfo.SkcInfo[i].SkuInfo[j].InventoryInfo[k] = LocalWarehouseInventory{
						WarehouseCode:     warehouseInfo.MerchantWarehouseCode,
						UsableInventory:   warehouseInfo.UsableInventory,
						InventoryQuantity: warehouseInfo.InventoryQuantity,
					}
				}
			}
		}
	}

	skuInventoryMap := inventoryManager.BuildSkuInventoryMap(localInventoryInfo)

	// 遍历所有 SKC 和 SKU，查询映射关系并填充到 SKU 级别
	foundMapping := false
	var firstAsin string
	var firstParentAsin string
	mappingCount := 0

	// 创建增强的 SKC 数据结构
	enrichedSkcList := make([]modules.EnrichedSkcInfo, 0, len(sheinProduct.SkcInfoList))

	for _, skc := range sheinProduct.SkcInfoList {
		enrichedSkc := modules.EnrichedSkcInfo{
			SkcName:               skc.SkcName,
			SkcCode:               skc.SkcCode,
			SaleName:              skc.SaleName,
			MainImageThumbnailURL: skc.MainImageThumbnailURL,
			SupplierCode:          skc.SupplierCode,
			BusinessModel:         skc.BusinessModel,
			IsSaleAttribute:       skc.IsSaleAttribute,
			SupplierID:            skc.SupplierID,
			SkuInfo:               make([]modules.EnrichedSkuInfo, 0, len(skc.SkuInfo)),
			MallSellStatus:        skc.MallSellStatus,
			Abandoned:             skc.Abandoned,
			TagInfoList:           skc.TagInfoList,
			ShelfFailReason:       skc.ShelfFailReason,
			HasOriginalImage:      skc.HasOriginalImage,
		}

		// 遍历 SKC 下的每个 SKU，为每个 SKU 查询映射关系、价格/成本价和库存
		for _, sku := range skc.SkuInfo {
			enrichedSku := e.buildEnrichedSkuInfo(sku, priceMap, costMap, skuInventoryMap, tenantID, storeID)
			enrichedSkc.SkuInfo = append(enrichedSkc.SkuInfo, enrichedSku)

			// 记录第一个找到的 ASIN 作为产品级别的 ProductID
			if enrichedSku.MappingInfo != nil {
				mappingCount++
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

	// 使用第一个找到的 ASIN 填充产品级别的 ProductID 和 Region
	if foundMapping {
		productData.ProductID = firstAsin
		if firstParentAsin != "" {
			productData.ParentProductID = firstParentAsin
		}

		// 从映射信息中获取 Region
		e.fillProductRegion(productData, enrichedSkcList)

		// 更新 Attributes，包含 SKU 级别的映射信息
		e.updateAttributesWithMappings(productData, enrichedSkcList)
	} else {
		e.logger.WithFields(logrus.Fields{
			"spu_code": sheinProduct.SpuCode,
			"store_id": storeID,
		}).Debug("未找到任何 SKU 映射关系")
	}
}

// buildEnrichedSkuInfo 构建增强的SKU信息
func (e *DataEnricher) buildEnrichedSkuInfo(sku SkuInfo, priceMap map[string]*product.SkuPriceInfo, costMap map[string]*product.SkuCostInfo, skuInventoryMap map[string]*LocalSkuInventory, tenantID, storeID int64) modules.EnrichedSkuInfo {
	enrichedSku := modules.EnrichedSkuInfo{
		SkuInfo: product.SkuInfo{
			SkuCode: sku.SkuCode,
		},
		MappingInfo:       nil,
		SaleNameInfo:      nil,
		PriceInfoList:     nil,
		SaleAttributeList: nil,
		CostPriceInfo:     nil,
		UsableInventory:   nil,
		InventoryQuantity: nil,
	}

	// 填充 SKU 级别的完整价格信息（自营店铺）
	if skuPriceInfo, ok := priceMap[sku.SkuCode]; ok {
		enrichedSku.SaleNameInfo = skuPriceInfo.SaleNameInfo
		enrichedSku.PriceInfoList = skuPriceInfo.PriceInfoList
	}

	// 填充 SKU 级别的完整成本价信息（半托店铺）
	if skuCostInfo, ok := costMap[sku.SkuCode]; ok {
		enrichedSku.SaleAttributeList = skuCostInfo.SaleAttributeList
		enrichedSku.CostPriceInfo = &skuCostInfo.CostPriceInfo
	}

	// 填充 SKU 级别的库存信息
	if skuInv, ok := skuInventoryMap[sku.SkuCode]; ok {
		// 转换本地库存信息为产品库存信息格式
		enrichedSku.InventoryInfo = make([]product.WarehouseInventory, len(skuInv.InventoryInfo))
		for i, warehouse := range skuInv.InventoryInfo {
			enrichedSku.InventoryInfo[i] = product.WarehouseInventory{
				MerchantWarehouseCode: warehouse.WarehouseCode,
				UsableInventory:       warehouse.UsableInventory,
				InventoryQuantity:     warehouse.InventoryQuantity,
			}
		}

		// 计算总库存和可用库存
		totalUsable := 0
		totalInventory := 0
		for _, warehouse := range skuInv.InventoryInfo {
			totalUsable += warehouse.UsableInventory
			totalInventory += warehouse.InventoryQuantity
		}
		enrichedSku.UsableInventory = &totalUsable
		enrichedSku.InventoryQuantity = &totalInventory
	}

	// 通过平台产品ID（SkuCode）查询映射关系
	mapping, err := e.mappingClient.GetProductImportMappingByPlatformProductIdAndStore(&api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{
		PlatformProductId: sku.SkuCode,
		StoreId:           storeID,
	})

	if err != nil {
		e.logger.WithError(err).WithFields(logrus.Fields{
			"sku_code": sku.SkuCode,
			"store_id": storeID,
		}).Debug("查询 SKU 映射关系失败")
	} else if mapping != nil {
		// 将映射信息添加到 SKU 级别
		enrichedSku.MappingInfo = mapping
	} else {
		e.logger.WithFields(logrus.Fields{
			"sku_code": sku.SkuCode,
			"store_id": storeID,
		}).Debug("未找到 SKU 映射关系")
	}

	return enrichedSku
}

// fillProductRegion 填充产品区域信息
func (e *DataEnricher) fillProductRegion(productData *api.ProductDataDTO, enrichedSkcList []modules.EnrichedSkcInfo) {
	for _, enrichedSkc := range enrichedSkcList {
		for _, enrichedSku := range enrichedSkc.SkuInfo {
			if enrichedSku.MappingInfo != nil && enrichedSku.MappingInfo.Region != "" {
				productData.Region = enrichedSku.MappingInfo.Region
				return
			}
		}
	}
}

// updateAttributesWithMappings 更新 Attributes，包含 SKU 级别的映射信息
func (e *DataEnricher) updateAttributesWithMappings(productData *api.ProductDataDTO, enrichedSkcList []modules.EnrichedSkcInfo) {
	// 序列化包含映射信息的 SKC 列表到 Attributes
	if attributesJSON, err := json.Marshal(enrichedSkcList); err == nil {
		productData.Attributes = string(attributesJSON)
		e.logger.Debug("已更新 Attributes，包含 SKU 级别的映射信息")
	} else {
		e.logger.WithError(err).Warn("序列化增强的 Attributes 失败")
	}
}
