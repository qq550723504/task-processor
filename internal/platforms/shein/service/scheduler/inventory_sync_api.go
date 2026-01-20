// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// delistProductViaSHEINAPI 通过SHEIN API下架产品
func (s *inventorySyncServiceImpl) delistProductViaSHEINAPI(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	storeID int64,
) error {
	spuName := prod.PlatformProductID
	s.logger.WithFields(logrus.Fields{
		"product_id": prod.ProductID,
		"spu_name":   spuName,
	}).Info("开始通过SHEIN API下架产品")

	// 获取店铺站点信息
	siteAbbr, err := s.getStoreSiteAbbr(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺站点信息失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"store_id":  storeID,
		"site_abbr": siteAbbr,
	}).Info("使用站点信息")

	// 解析 Attributes 获取 SKC 信息
	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(prod.Attributes), &skcList); err != nil {
		return fmt.Errorf("解析产品attributes失败: %w", err)
	}

	if len(skcList) == 0 {
		return fmt.Errorf("产品没有SKC信息")
	}

	// 构建下架请求
	skcSiteInfos := make([]product.SkcSiteInfo, 0, len(skcList))
	for _, skc := range skcList {
		skcSiteInfo := product.SkcSiteInfo{
			BusinessModel: skc.BusinessModel,
			SkcName:       skc.SkcName,
			OffSubSites: []product.SubSite{
				{
					SiteAbbr:  siteAbbr,
					StoreType: 1,
				},
			},
		}
		skcSiteInfos = append(skcSiteInfos, skcSiteInfo)
	}

	offShelfReq := &product.ShelfOperateRequest{
		SpuName:      spuName,
		SkcSiteInfos: skcSiteInfos,
	}

	// 调用SHEIN API下架
	if err := s.productAPI.OffShelf(offShelfReq); err != nil {
		return fmt.Errorf("调用SHEIN API下架产品失败: %w", err)
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
		ProductStock:       0, // 库存设为0，因为已下架
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

	productDataAPI := s.managementClient.GetProductDataClient(storeID)
	if _, err := productDataAPI.BatchCreateOrUpdate(batchReq); err != nil {
		s.logger.WithError(err).Warn("更新管理系统中的产品状态失败")
	}

	s.logger.WithFields(logrus.Fields{
		"product_id": prod.ProductID,
		"spu_name":   spuName,
	}).Info("产品已成功通过SHEIN API下架")

	return nil
}

// updateProductStockViaSHEINAPI 通过SHEIN API更新产品库存
func (s *inventorySyncServiceImpl) updateProductStockViaSHEINAPI(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *SKUMappingData,
	targetStock int,
	storeID int64,
) error {
	spuName := prod.PlatformProductID
	platformSKU := s.getStringValue(skuMapping.MappingInfo.Sku)

	s.logger.WithFields(logrus.Fields{
		"product_id":   prod.ProductID,
		"spu_name":     spuName,
		"sku":          platformSKU,
		"target_stock": targetStock,
	}).Info("开始通过SHEIN API更新产品库存")

	// 解析 Attributes 获取 SKC/SKU 信息
	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(prod.Attributes), &skcList); err != nil {
		return fmt.Errorf("解析产品attributes失败: %w", err)
	}

	// 查找对应的SKC和SKU
	var targetSkcCode, targetSkcName, targetSkuCode string
	found := false

	for _, skc := range skcList {
		for _, sku := range skc.SkuInfo {
			if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
				targetSkcCode = skc.SkcCode
				targetSkcName = skc.SkcName
				targetSkuCode = sku.SkuCode
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到对应的SKU: %s", platformSKU)
	}

	// 从 SKU 的库存信息中获取仓库列表
	var warehouseUpdates []product.WarehouseInventoryUpdate

	// 查找对应 SKU 的库存信息
	for _, skc := range skcList {
		if skc.SkcCode == targetSkcCode {
			for _, sku := range skc.SkuInfo {
				if sku.SkuCode == targetSkuCode && len(sku.InventoryInfo) > 0 {
					// 为每个仓库构建更新信息
					for _, warehouse := range sku.InventoryInfo {
						warehouseUpdates = append(warehouseUpdates, product.WarehouseInventoryUpdate{
							MerchantWarehouseCode:    warehouse.MerchantWarehouseCode,
							BeforeChangeInventoryNum: warehouse.UsableInventory,
							AfterChangeInventoryNum:  targetStock,
						})
					}
					break
				}
			}
			break
		}
	}

	// 如果没有找到仓库信息，返回错误
	if len(warehouseUpdates) == 0 {
		return fmt.Errorf("未找到SKU %s 的仓库信息，无法更新库存", platformSKU)
	}

	// 构建库存更新请求
	updateReq := &product.InventoryUpdateRequest{
		SkcInfo: []product.SkcInventoryUpdate{
			{
				SkcCode: targetSkcCode,
				SkcName: targetSkcName,
				SkuInfo: []product.SkuInventoryUpdate{
					{
						SkuCode:           targetSkuCode,
						DeliveryMode:      1,
						WarehouseInfoList: warehouseUpdates,
					},
				},
			},
		},
	}

	// 调用SHEIN API更新库存
	if err := s.productAPI.UpdateInventory(updateReq); err != nil {
		return fmt.Errorf("调用SHEIN API更新库存失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":   prod.ProductID,
		"spu_name":     spuName,
		"sku":          platformSKU,
		"target_stock": targetStock,
	}).Info("产品库存已成功通过SHEIN API更新")

	return nil
}
