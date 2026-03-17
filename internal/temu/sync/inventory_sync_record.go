// package sync 提供TEMU平台库存和价格记录逻辑
package sync

import (
	"encoding/json"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/pkg/timex"
	"task-processor/internal/product"
	"time"

	"github.com/sirupsen/logrus"
)

// recordInventoryAndPrice 记录库存和价格历史（每天一次）- 参考SHEIN实现
func (s *inventorySyncServiceImpl) recordInventoryAndPrice(
	productId, region string,
	amazonProduct *model.Product,
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuSkuInfo,
	storeID int64,
) {
	defer recovery.Recover("记录TEMU库存和价格历史", s.logger)

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
		if timex.IsSameDate(latestRecord.CreateTime.Time, time.Now()) {
			s.logger.WithFields(logrus.Fields{
				"productId": productId,
				"region":    region,
			}).Debug("今天已记录过库存价格，跳过")
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
		var mappingList []TemuMappingData
		if err := jsonx.UnmarshalString(prod.Attributes, &mappingList, ""); err == nil {
			// 查找对应的 SKU
			for _, mapping := range mappingList {
				for _, sku := range mapping.SkuInfo {
					if s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
						// 找到对应的 SKU，从 AmazonMonitorData 中获取历史价格
						if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
							originalPrice = sku.AmazonMonitorData.Price
							s.logger.WithFields(logrus.Fields{
								"platform_sku":   platformSKU,
								"original_price": originalPrice,
							}).Debug("从TEMU产品Attributes中获取到历史价格")
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
		}).Debug("TEMU首次记录，使用当前价格作为原价")
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
		SyncSource:         "TEMU_MONITOR",
		Remark:             "TEMU库存监控自动记录",
	}

	if recordID, err := s.inventoryRecordClient.CreateInventoryRecord(recordReq); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"product_id": productId,
			"region":     region,
		}).Error("创建TEMU库存价格记录失败")
	} else {
		s.logger.WithFields(logrus.Fields{
			"product_id":     productId,
			"region":         region,
			"record_id":      recordID,
			"stock":          stock,
			"price":          currentPrice,
			"price_type":     priceType,
			"original_price": originalPrice,
		}).Info("已记录TEMU库存和价格变动")
	}
}

// updateAttributesWithAmazonData 更新产品attributes中的Amazon数据（异步）- 参考SHEIN实现
func (s *inventorySyncServiceImpl) updateAttributesWithAmazonData(
	prod *managementapi.ProductDataDTO,
	platformSKU string,
	amazonProduct *model.Product,
	storeID int64,
) {
	defer recovery.Recover("更新TEMU Attributes", s.logger)

	s.logger.WithFields(logrus.Fields{
		"product_id":   prod.ProductID,
		"platform_sku": platformSKU,
		"asin":         amazonProduct.Asin,
	}).Debug("开始更新TEMU产品Attributes中的Amazon数据")

	if prod.Attributes == "" {
		s.logger.WithField("product_id", prod.ProductID).Warn("TEMU产品Attributes为空，无法更新Amazon数据")
		return
	}

	var mappingList []TemuMappingData
	if err := jsonx.UnmarshalString(prod.Attributes, &mappingList, "解析TEMU产品attributes失败"); err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error(err.Error())
		return
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(storeID)

	// 查找对应的SKU并更新Amazon监控数据
	updated := false
	for i := range mappingList {
		for j := range mappingList[i].SkuInfo {
			sku := &mappingList[i].SkuInfo[j]
			if s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
				newStock := s.extractStockFromProduct(amazonProduct)

				// 使用公共函数获取价格（根据店铺配置的价格类型）
				currentPrice := product.GetProductPrice(amazonProduct, priceType)

				// 更新Amazon监控数据
				sku.AmazonMonitorData = &TemuAmazonMonitorData{
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
				}).Debug("已更新TEMU SKU的Amazon监控数据")
				updated = true
				break
			}
		}
		if updated {
			break
		}
	}

	if !updated {
		s.logger.WithFields(logrus.Fields{
			"product_id":   prod.ProductID,
			"platform_sku": platformSKU,
		}).Warn("未找到对应的TEMU SKU，跳过更新")
		return
	}

	// 序列化并保存
	updatedAttributes, err := json.Marshal(mappingList)
	if err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("序列化更新后的TEMU attributes失败")
		return
	}

	// 使用批量更新attributes接口
	productDataAPI := s.managementClient.GetProductDataClient(storeID)

	updateReq := &managementapi.ProductDataBatchUpdateAttributesReqDTO{
		Platform: "TEMU",
		TenantID: prod.TenantID,
		StoreID:  storeID,
		Region:   prod.Region,
		Products: []managementapi.ProductAttributesItemDTO{
			{
				PlatformProductID: prod.PlatformProductID,
				Attributes:        string(updatedAttributes),
				UpdateTime:        &[]int64{time.Now().Unix()}[0],
			},
		},
	}

	if count, err := productDataAPI.BatchUpdateAttributes(updateReq); err != nil {
		s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("更新TEMU产品attributes失败")
	} else {
		if count <= 0 {
			s.logger.WithField("product_id", prod.ProductID).Warn("未更新任何TEMU产品attributes")
		} else {
			s.logger.WithField("product_id", prod.ProductID).Info("已更新TEMU产品attributes中的Amazon监控数据")
		}
	}
}
