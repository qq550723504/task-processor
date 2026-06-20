// Package inventory 提供 SHEIN 平台库存同步功能
package inventory

import (
	"context"
	"encoding/json"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/pkg/timex"
	"task-processor/internal/product"
	"task-processor/internal/shein/productsync"
	"time"

	"github.com/sirupsen/logrus"
)

// recordInventoryAndPrice 记录库存和价格历史（每天一次）
func (s *inventorySyncServiceImpl) recordInventoryAndPrice(
	productId, region string,
	amazonProduct *model.Product,
	prod *InventoryProductSnapshot,
	skuMapping *SKUMappingData,
	storeID int64,
) {
	defer recovery.Recover("记录库存和价格", s.logger)

	if amazonProduct == nil || skuMapping == nil || skuMapping.MappingInfo == nil {
		s.logger.WithFields(logrus.Fields{
			"productId":          productId,
			"amazonProductIsNil": amazonProduct == nil,
			"skuMappingIsNil":    skuMapping == nil,
		}).Warn("recordInventoryAndPrice: 参数为nil，跳过")
		return
	}

	// 检查今天是否已经记录过
	if s.inventoryRecordRepo == nil {
		s.logger.WithField("productId", productId).Warn("inventory record repository is not configured, skip record")
		return
	}
	latestRecord, err := s.inventoryRecordRepo.GetLatestInventoryRecord(context.Background(), "Amazon", productId, region)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"productId": productId,
			"region":    region,
		}).Warn("查询最新库存记录失败")
	}

	// 如果今天已经记录过，跳过
	if latestRecord != nil {
		if latestRecord.CreateTime != nil && timex.IsSameDate(*latestRecord.CreateTime, time.Now()) {
			return
		}
	}

	// 提取库存和价格信息
	stock := s.extractStockFromProduct(amazonProduct)

	// 从产品 Attributes 中获取该 SKU 的历史价格作为原价
	var originalPrice float64
	platformSKU := s.getStringValue(skuMapping.MappingInfo.SKU)

	// 解析 Attributes 获取 SKU 的 AmazonMonitorData
	if prod.Attributes != "" {
		var skcList []productsync.EnrichedSkcInfo
		if err := jsonx.UnmarshalString(prod.Attributes, &skcList, ""); err == nil {
			// 查找对应的 SKU
			for _, skc := range skcList {
				for _, sku := range skc.SkuInfo {
					if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.SKU) == platformSKU {
						// 找到对应的 SKU，从 AmazonMonitorData 中获取历史价格
						if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
							originalPrice = sku.AmazonMonitorData.Price
							s.logger.WithFields(logrus.Fields{
								"platform_sku":   platformSKU,
								"original_price": originalPrice,
							}).Debug("从产品Attributes中获取到历史价格")
						}
						break
					}
				}
			}
		}
	}

	// 如果没有找到历史价格，使用当前价格作为原价（首次记录）
	priceType := s.getStorePriceType(storeID)
	currentPrice := product.GetProductPrice(amazonProduct, priceType)

	if originalPrice == 0 {
		originalPrice = currentPrice
		s.logger.WithFields(logrus.Fields{
			"platform_sku": platformSKU,
			"price":        currentPrice,
		}).Debug("首次记录，使用当前价格作为原价")
	}

	currency := amazonProduct.Currency

	// 计算价格变化百分比
	var priceChangePercent *float64
	if latestRecord != nil && latestRecord.CurrentPrice != nil && *latestRecord.CurrentPrice > 0 {
		change := ((currentPrice - *latestRecord.CurrentPrice) / *latestRecord.CurrentPrice) * 100
		priceChangePercent = &change
	}

	// 创建库存记录
	recordReq := &listingadmin.InventoryRecord{
		Platform:           "Amazon",
		ProductID:          productId,
		Region:             region,
		Stock:              &stock,
		StockStatus:        amazonProduct.Availability,
		IsAvailable:        amazonProduct.IsAvailable,
		OriginalPrice:      &originalPrice,
		CurrentPrice:       &currentPrice,
		Currency:           currency,
		PriceChangePercent: priceChangePercent,
		SyncSource:         "MONITOR",
		Remark:             "库存监控自动记录",
	}

	if record, err := s.inventoryRecordRepo.CreateInventoryRecord(context.Background(), recordReq); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"product_id": productId,
			"region":     region,
		}).Error("创建库存价格记录失败")
	} else {
		s.logger.WithFields(logrus.Fields{
			"product_id":     productId,
			"region":         region,
			"record_id":      record.ID,
			"stock":          stock,
			"price":          currentPrice,
			"price_type":     priceType,
			"original_price": originalPrice,
		}).Info("已记录库存和价格变动")
	}
}

// updateAttributesWithAmazonData 更新产品attributes中的Amazon数据（异步）
func (s *inventorySyncServiceImpl) updateAttributesWithAmazonData(
	prod *InventoryProductSnapshot,
	platformSKU string,
	amazonProduct *model.Product,
	storeID int64,
) {
	defer recovery.Recover("更新Attributes", s.logger)

	var skcList []productsync.EnrichedSkcInfo
	if err := jsonx.UnmarshalString(prod.Attributes, &skcList, "解析产品attributes失败"); err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error(err.Error())
		return
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(storeID)

	// 查找对应的SKU并更新Amazon监控数据
	updated := false
	for i := range skcList {
		for j := range skcList[i].SkuInfo {
			sku := &skcList[i].SkuInfo[j]
			if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.SKU) == platformSKU {
				newStock := s.extractStockFromProduct(amazonProduct)

				// 使用公共函数获取价格（根据店铺配置的价格类型）
				currentPrice := product.GetProductPrice(amazonProduct, priceType)

				// 更新Amazon监控数据
				sku.AmazonMonitorData = &AmazonMonitorData{
					ASIN:          amazonProduct.Asin,
					Price:         currentPrice,
					Stock:         newStock,
					LastCheckTime: time.Now().Unix(),
				}

				s.logger.WithFields(logrus.Fields{
					"platform_sku": platformSKU,
					"asin":         amazonProduct.Asin,
					"price":        currentPrice,
					"price_type":   priceType,
					"stock":        newStock,
				}).Debug("已更新SKU的Amazon监控数据")
				updated = true
				break
			}
		}
		if updated {
			break
		}
	}

	if !updated {
		s.logger.WithField("platform_sku", platformSKU).Debug("未找到对应的SKU，跳过更新")
		return
	}

	// 序列化并保存
	updatedAttributes, err := json.Marshal(skcList)
	if err != nil {
		s.logger.WithError(err).Error("序列化更新后的attributes失败")
		return
	}

	if count, err := s.updateInventoryProductAttributes(context.Background(), prod, string(updatedAttributes)); err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("更新产品attributes失败")
	} else {
		if count <= 0 {
			s.logger.WithField("product_id", prod.ProductID).Warn("未更新任何产品attributes")
		} else {
			s.logger.WithField("product_id", prod.ProductID).Debug("已更新产品attributes")
		}
	}
}
