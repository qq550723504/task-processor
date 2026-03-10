// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"encoding/json"
	"time"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/pkg/jsonutil"
	managementapi "task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// recordInventoryAndPrice 记录库存和价格历史（每天一次）
func (s *inventorySyncServiceImpl) recordInventoryAndPrice(
	productId, region string,
	amazonProduct *model.Product,
	prod *managementapi.ProductDataDTO,
	skuMapping *SKUMappingData,
	storeID int64,
) {
	// 检查今天是否已经记录过
	latestRecord, err := s.inventoryRecordClient.GetLatestInventoryRecord("Amazon", productId, region)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"productId": productId,
			"region":    region,
		}).Warn("查询最新库存记录失败")
	}

	// 如果今天已经记录过，跳过
	if latestRecord != nil {
		recordDate := latestRecord.CreateTime.Format("2006-01-02")
		today := time.Now().Format("2006-01-02")
		if recordDate == today {
			return
		}
	}

	// 提取库存和价格信息
	stock := s.extractStockFromProduct(amazonProduct)

	// 从产品 Attributes 中获取该 SKU 的历史价格作为原价
	var originalPrice float64
	platformSKU := s.getStringValue(skuMapping.MappingInfo.Sku)

	// 解析 Attributes 获取 SKU 的 AmazonMonitorData
	if prod.Attributes != "" {
		var skcList []EnrichedSkcInfo
		if err := jsonutil.UnmarshalString(prod.Attributes, &skcList, ""); err == nil {
			// 查找对应的 SKU
			for _, skc := range skcList {
				for _, sku := range skc.SkuInfo {
					if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
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
	recordReq := &managementapi.InventoryRecordCreateReqDTO{
		Platform:           "Amazon",
		ProductId:          productId,
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

	if recordID, err := s.inventoryRecordClient.CreateInventoryRecord(recordReq); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"product_id": productId,
			"region":     region,
		}).Error("创建库存价格记录失败")
	} else {
		s.logger.WithFields(logrus.Fields{
			"product_id":     productId,
			"region":         region,
			"record_id":      recordID,
			"stock":          stock,
			"price":          currentPrice,
			"price_type":     priceType,
			"original_price": originalPrice,
		}).Info("已记录库存和价格变动")
	}
}

// updateAttributesWithAmazonData 更新产品attributes中的Amazon数据（异步）
func (s *inventorySyncServiceImpl) updateAttributesWithAmazonData(
	prod *managementapi.ProductDataDTO,
	platformSKU string,
	amazonProduct *model.Product,
	storeID int64,
) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.WithField("panic", r).Error("更新Attributes时发生panic")
		}
	}()

	var skcList []EnrichedSkcInfo
	if err := jsonutil.UnmarshalString(prod.Attributes, &skcList, "解析产品attributes失败"); err != nil {
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
			if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
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

	// 使用批量更新attributes接口
	productDataAPI := s.managementClient.GetProductDataClient(storeID)

	updateReq := &managementapi.ProductDataBatchUpdateAttributesReqDTO{
		Platform: "SHEIN",
		TenantID: prod.TenantID,
		StoreID:  storeID,
		Region:   prod.Region,
		Products: []managementapi.ProductAttributesItemDTO{
			{
				PlatformProductID: prod.PlatformProductID,
				Attributes:        string(updatedAttributes),
			},
		},
	}

	if count, err := productDataAPI.BatchUpdateAttributes(updateReq); err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("更新产品attributes失败")
	} else {
		if count <= 0 {
			s.logger.WithField("product_id", prod.ProductID).Warn("未更新任何产品attributes")
		} else {
			s.logger.WithField("product_id", prod.ProductID).Debug("已更新产品attributes")
		}
	}
}
