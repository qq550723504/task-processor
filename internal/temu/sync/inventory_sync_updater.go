// package sync 提供TEMU平台库存更新相关服务
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/product"
	"time"

	"github.com/sirupsen/logrus"
)

// batchUpdateTemuInventoryInAttributes 批量更新TEMU平台attributes中的库存和Amazon监控数据
func (s *inventorySyncServiceImpl) batchUpdateTemuInventoryInAttributes(
	_ context.Context,
	batch *InventoryUpdateBatch,
) error {
	defer recovery.Recover("批量更新TEMU库存", s.logger)

	prod := batch.Product
	updates := batch.Updates
	storeID := batch.StoreID

	s.logger.WithFields(logrus.Fields{
		"product_id":   prod.ProductID,
		"update_count": len(updates),
	}).Debug("开始批量更新TEMU产品Attributes中的库存和Amazon监控数据")

	if prod.Attributes == "" {
		s.logger.WithField("product_id", prod.ProductID).Warn("TEMU产品Attributes为空，无法更新库存数据")
		return fmt.Errorf("产品Attributes为空")
	}

	// 解析现有的attributes数据
	var mappingList []TemuMappingData
	if err := jsonx.UnmarshalString(prod.Attributes, &mappingList, "解析TEMU产品attributes失败"); err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error(err.Error())
		return fmt.Errorf("解析产品attributes失败: %w", err)
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(storeID)

	// 创建SKU到更新信息的映射，便于快速查找
	updateMap := make(map[string]*SkuInventoryUpdate)
	for i := range updates {
		updateMap[updates[i].PlatformSKU] = &updates[i]
	}

	// 批量更新所有匹配的SKU库存和Amazon监控数据
	updatedCount := 0
	for i := range mappingList {
		for j := range mappingList[i].SkuInfo {
			sku := &mappingList[i].SkuInfo[j]
			platformSKU := s.getStringValue(sku.MappingInfo.Sku)

			if update, exists := updateMap[platformSKU]; exists {
				// 更新库存数量（使用Amazon的库存数据）
				oldInventory := sku.UsableInventory
				newInventory := int(update.NewInventory)
				sku.UsableInventory = newInventory

				// 更新Amazon监控数据
				amazonProduct := update.AmazonProduct
				currentPrice := product.GetProductPrice(amazonProduct, priceType)
				newStock := s.extractStockFromProduct(amazonProduct)

				sku.AmazonMonitorData = &TemuAmazonMonitorData{
					ASIN:          amazonProduct.Asin,
					Price:         currentPrice,
					Stock:         newStock,
					LastCheckTime: time.Now().Unix(),
				}

				s.logger.WithFields(logrus.Fields{
					"platform_sku":  platformSKU,
					"old_inventory": oldInventory,
					"new_inventory": newInventory,
					"amazon_stock":  newStock,
					"amazon_price":  currentPrice,
					"price_type":    priceType,
					"asin":          amazonProduct.Asin,
				}).Debug("已更新TEMU SKU的库存和Amazon监控数据")

				updatedCount++
			}
		}
	}

	if updatedCount == 0 {
		s.logger.WithField("product_id", prod.ProductID).Warn("未找到任何匹配的TEMU SKU，跳过库存更新")
		return fmt.Errorf("未找到任何匹配的SKU")
	}

	// 序列化更新后的数据
	updatedAttributes, err := json.Marshal(mappingList)
	if err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("序列化更新后的TEMU attributes失败")
		return fmt.Errorf("序列化attributes失败: %w", err)
	}

	count, err := s.updateInventoryProductAttributes(context.Background(), prod, string(updatedAttributes))
	if err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("批量更新TEMU产品attributes失败")
		return fmt.Errorf("批量更新产品attributes失败: %w", err)
	}

	if count <= 0 {
		s.logger.WithField("product_id", prod.ProductID).Warn("未更新任何TEMU产品attributes")
		return fmt.Errorf("未更新任何产品attributes")
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":    prod.ProductID,
		"updated_count": count,
		"sku_count":     updatedCount,
		"total_updates": len(updates),
	}).Info("成功批量更新TEMU产品attributes中的库存和Amazon监控数据")

	return nil
}
