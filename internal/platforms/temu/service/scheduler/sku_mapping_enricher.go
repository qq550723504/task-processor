// Package scheduler 提供TEMU平台SKU映射增强功能
package scheduler

import (
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// buildEnrichedMappingData 构建增强的映射数据，为每个SKU创建独立的映射对象（SHEIN格式）
func (s *productSyncServiceImpl) buildEnrichedMappingData(
	temuProduct *models.GoodsSearchItem,
	skuDetails *models.SkuQueryResponse,
	storeID int64,
) ([]*models.TemuMappingData, error) {
	if s.mappingClient == nil {
		return nil, fmt.Errorf("映射客户端未设置")
	}

	// 为每个SKU创建独立的映射对象
	var result []*models.TemuMappingData

	for i, skuItem := range skuDetails.Result.SkuList {
		// 为每个SKU创建独立的映射数据对象
		skuInfo := models.TemuSkuInfo{
			SkuCode: skuItem.SkuID,
			// 初始化空的映射信息，后续会通过查询填充
			MappingInfo: models.TemuMappingInfo{},
			// 填充成本价格信息
			CostPriceInfo: models.TemuCostPriceInfo{
				Currency:  skuItem.Currency, // 使用SKU的货币
				CostPrice: fmt.Sprintf("%.2f", skuItem.Price),
			},
			// 填充TEMU库存信息
			// TemuInventoryInfo: models.TemuInventoryInfo{
			// 	UsableInventory: skuItem.Stock, // 使用SKU的库存数量
			// },
			UsableInventory: skuItem.Stock,
		}

		// 使用SkuSN查询SKU的映射关系
		mapping, err := s.mappingClient.GetProductImportMappingBySku(&managementapi.ProductImportMappingGetBySkuReqDTO{
			Sku:     skuItem.SkuSN, // 使用SkuSN而不是OutGoodsSN
			StoreId: storeID,
		})

		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"sku_id":   skuItem.SkuID,
				"sku_sn":   skuItem.SkuSN,
				"store_id": storeID,
			}).Warn("查询SKU映射关系失败")
			// 继续处理其他SKU，不中断整个流程
		} else if mapping != nil {
			// 将查询到的映射信息填充到SKU中
			skuInfo.MappingInfo = models.TemuMappingInfo{
				ID:                      mapping.ID,
				ImportTaskId:            mapping.ImportTaskId,
				StoreId:                 mapping.StoreId,
				Platform:                mapping.Platform,
				Region:                  mapping.Region,
				ProductId:               mapping.ProductId,
				ParentProductId:         mapping.ParentProductId,
				PlatformProductId:       mapping.PlatformProductId,
				PlatformParentProductId: mapping.PlatformParentProductId,
				Sku:                     mapping.Sku,
				CostPrice:               mapping.CostPrice,
				FilterRuleId:            mapping.FilterRuleId,
				FilterRuleRange:         mapping.FilterRuleRange,
				ProfitRuleId:            mapping.ProfitRuleId,
				SalePriceMultiplier:     mapping.SalePriceMultiplier,
				DiscountPriceMultiplier: mapping.DiscountPriceMultiplier,
				Status:                  mapping.Status,
				Remark:                  mapping.Remark,
				TenantId:                mapping.TenantId,
			}

		} else {
			s.logger.WithFields(logrus.Fields{
				"sku_id":   skuItem.SkuID,
				"sku_sn":   skuItem.SkuSN,
				"store_id": storeID,
			}).Warnf("未找到SKU映射关系")
		}

		// 为每个SKU创建独立的映射数据对象（SHEIN格式）
		mappingData := &models.TemuMappingData{
			SkcCode:               skuItem.SkuID,                                  // 使用SKU ID作为skc_code
			SkcName:               fmt.Sprintf("%s_%d", temuProduct.GoodsID, i+1), // 生成唯一的skc_name
			SkuInfo:               []models.TemuSkuInfo{skuInfo},                  // 每个对象只包含一个SKU
			SaleName:              skuItem.SpecName,                               // 使用SKU的SpecName作为SaleName
			SupplierID:            0,                                              // 需要根据实际情况填充
			SupplierCode:          skuItem.SkuSN,                                  // 使用SkuSN作为SupplierCode
			MallSellStatus:        1,                                              // 默认在售状态
			MainImageThumbnailURL: skuItem.ThumbURL,                               // 使用SKU的ThumbURL
		}

		result = append(result, mappingData)
	}

	return result, nil
}

// fillProductLevelMappingInfo 使用第一个找到的映射信息填充产品级别的ProductID和Region
func (s *productSyncServiceImpl) fillProductLevelMappingInfo(
	productData *managementapi.ProductDataDTO,
	mappingDataList []*models.TemuMappingData,
) {
	if len(mappingDataList) == 0 {
		return
	}

	// 查找第一个有效的映射信息
	for _, mappingData := range mappingDataList {
		if mappingData == nil {
			continue
		}

		for _, skuInfo := range mappingData.SkuInfo {
			if skuInfo.MappingInfo.ProductId != "" {
				productData.ProductID = skuInfo.MappingInfo.ProductId

				if skuInfo.MappingInfo.ParentProductId != nil && *skuInfo.MappingInfo.ParentProductId != "" {
					productData.ParentProductID = *skuInfo.MappingInfo.ParentProductId
				}

				if skuInfo.MappingInfo.Region != "" {
					productData.Region = skuInfo.MappingInfo.Region
				}

				return
			}
		}
	}

	s.logger.Debug("未找到有效的SKU映射信息，产品级别数据保持原值")
}
