// Package shein 提供SHEIN产品变化检测功能
package shein

import (
	"time"

	"task-processor/internal/common"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management/api"
	"task-processor/internal/platforms/shein/modules"

	"github.com/sirupsen/logrus"
)

// ChangeDetector 变化检测器
type ChangeDetector struct {
	config       *common.MonitorConfig
	eventHandler common.MonitorEventHandler
}

// NewChangeDetector 创建变化检测器
func NewChangeDetector(config *common.MonitorConfig, eventHandler common.MonitorEventHandler) *ChangeDetector {
	return &ChangeDetector{
		config:       config,
		eventHandler: eventHandler,
	}
}

// CheckAndNotifyPriceChange 检查并通知价格变化
func (d *ChangeDetector) CheckAndNotifyPriceChange(
	prod *api.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *SKUMappingData,
	tenantID, storeID int64,
	priceType string,
) bool {
	mappingInfo := skuMapping.MappingInfo

	// 使用 mappingInfo 中的成本价作为旧价格
	oldPrice := mappingInfo.CostPrice
	if oldPrice <= 0 {
		// 如果没有成本价，尝试使用产品级别的价格
		oldPrice = parsePrice(prod.OriginalPrice.String())
		if oldPrice <= 0 {
			oldPrice = parsePrice(prod.SpecialPrice.String())
		}
	}

	// 使用店铺配置的价格类型获取 Amazon 价格(包含运费)
	newPrice := modules.GetProductPrice(amazonProduct, priceType)

	if oldPrice > 0 && newPrice > 0 {
		changePercent := ((newPrice - oldPrice) / oldPrice) * 100

		if abs(changePercent) >= d.config.PriceChangeThreshold {
			event := &common.PriceChangeEvent{
				TenantID:          tenantID,
				StoreID:           storeID,
				Platform:          "SHEIN",
				ProductID:         mappingInfo.ProductID, // 使用 ASIN
				SKU:               mappingInfo.SKU,       // 使用平台 SKU
				OldPrice:          oldPrice,
				NewPrice:          newPrice,
				ChangePercent:     changePercent,
				PlatformProductID: prod.PlatformProductID,
				Timestamp:         time.Now(),
			}

			if err := d.eventHandler.OnPriceChange(event); err != nil {
				logrus.WithError(err).Error("处理价格变化事件失败")
				return false
			}
			return true
		}
	}

	return false
}

// CheckAndNotifyStockChange 检查并通知库存变化
func (d *ChangeDetector) CheckAndNotifyStockChange(
	prod *api.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *SKUMappingData,
	tenantID, storeID int64,
) bool {
	mappingInfo := skuMapping.MappingInfo

	// 使用 SKU 级别的库存
	oldStock := skuMapping.Stock
	newStock := extractStockFromProduct(amazonProduct)

	changeAmount := newStock - oldStock

	if absInt(changeAmount) >= d.config.StockChangeThreshold {
		event := &common.StockChangeEvent{
			TenantID:          tenantID,
			StoreID:           storeID,
			Platform:          "SHEIN",
			ProductID:         mappingInfo.ProductID, // 使用 ASIN
			SKU:               mappingInfo.SKU,       // 使用平台 SKU
			OldStock:          oldStock,
			NewStock:          newStock,
			ChangeAmount:      changeAmount,
			PlatformProductID: prod.PlatformProductID,
			Timestamp:         time.Now(),
		}

		if err := d.eventHandler.OnStockChange(event); err != nil {
			logrus.WithError(err).Error("处理库存变化事件失败")
			return false
		}
		return true
	}

	return false
}
