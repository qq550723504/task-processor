// package sync 提供TEMU平台SKU映射增强功能
package sync

import (
	"context"
	"fmt"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
	temuproduct "task-processor/internal/temu/api/product"
	temuquery "task-processor/internal/temu/api/query"

	"github.com/sirupsen/logrus"
)

// buildEnrichedMappingData 构建增强的映射数据，为每个SKU创建独立的映射对象（SHEIN格式）
func (s *productSyncServiceImpl) buildEnrichedMappingData(
	temuProduct *temuproduct.GoodsSearchItem,
	skuDetails *temuquery.SkuQueryResponse,
	storeID int64,
) ([]*TemuMappingData, error) {
	if s.mappingClient == nil && s.mappingRepo == nil {
		return nil, fmt.Errorf("映射客户端未设置")
	}

	// 为每个SKU创建独立的映射对象
	var result []*TemuMappingData

	for i, skuItem := range skuDetails.Result.SkuList {
		// 为每个SKU创建独立的映射数据对象
		skuInfo := TemuSkuInfo{
			SkuCode: skuItem.SkuID,
			// 初始化空的映射信息，后续会通过查询填充
			MappingInfo: TemuMappingInfo{},
			// 填充成本价格信息
			CostPriceInfo: TemuCostPriceInfo{
				Currency:  skuItem.Currency, // 使用SKU的货币
				CostPrice: fmt.Sprintf("%.2f", skuItem.Price),
			},
			UsableInventory: skuItem.Stock,
		}

		// 使用SkuSN查询SKU的映射关系
		mapping, err := s.getMappingBySKU(context.Background(), skuItem.SkuSN, storeID)

		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"sku_id":   skuItem.SkuID,
				"sku_sn":   skuItem.SkuSN,
				"store_id": storeID,
			}).Warn("查询SKU映射关系失败")
			// 继续处理其他SKU，不中断整个流程
		} else if mapping != nil {
			// 将查询到的映射信息填充到SKU中
			skuInfo.MappingInfo = TemuMappingInfo{
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
		mappingData := &TemuMappingData{
			SkcCode:               skuItem.SkuID,                                  // 使用SKU ID作为skc_code
			SkcName:               fmt.Sprintf("%s_%d", temuProduct.GoodsID, i+1), // 生成唯一的skc_name
			SkuInfo:               []TemuSkuInfo{skuInfo},                         // 每个对象只包含一个SKU
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

func (s *productSyncServiceImpl) getMappingBySKU(ctx context.Context, sku string, storeID int64) (*managementapi.ProductImportMappingRespDTO, error) {
	if s.mappingRepo != nil {
		mapping, err := s.mappingRepo.FindLatest(ctx, listingadmin.ProductImportMappingQuery{
			SKU:     sku,
			StoreID: &storeID,
		})
		if err == nil && mapping != nil {
			return temuProductImportMappingDTO(mapping), nil
		}
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"sku":      sku,
				"store_id": storeID,
				"path":     "repository",
			}).Warn("从本地仓储查询SKU映射失败，回退 management 接口")
		}
	}
	if s.mappingClient == nil {
		return nil, nil
	}
	return s.mappingClient.GetProductImportMappingBySku(&managementapi.ProductImportMappingGetBySkuReqDTO{
		Sku:     sku,
		StoreId: storeID,
	})
}

func temuProductImportMappingDTO(mapping *listingadmin.ProductImportMapping) *managementapi.ProductImportMappingRespDTO {
	if mapping == nil {
		return nil
	}
	return &managementapi.ProductImportMappingRespDTO{
		ID:           mapping.ID,
		ImportTaskId: mapping.ImportTaskID,
		StoreId:      mapping.StoreID,
		Platform:     mapping.Platform,
		Region:       mapping.Region,
		ProductId:    mapping.ProductID,
		CostPrice:    mapping.CostPrice,
		FilterRuleId: mapping.FilterRuleID,
		ProfitRuleId: mapping.ProfitRuleID,
		Status:       mapping.Status,
		TenantId:     mapping.TenantID,
	}
}

// fillProductLevelMappingInfo 使用第一个找到的映射信息填充产品级别的ProductID和Region
func (s *productSyncServiceImpl) fillProductLevelMappingInfo(
	productData *TemuProductSnapshot,
	mappingDataList []*TemuMappingData,
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

}
