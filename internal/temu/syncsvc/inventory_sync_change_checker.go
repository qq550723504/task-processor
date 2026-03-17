// Package syncsvc 提供TEMU平台库存监控变化检测逻辑
package syncsvc

import (
	"task-processor/internal/model"
	managementapi "task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// checkPriceChange 检查价格变化（基于利润率，参考SHEIN实现）
func (s *inventorySyncServiceImpl) checkPriceChange(
	prod *managementapi.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *TemuSkuInfo,
	storeID int64,
) bool {
	mappingInfo := skuMapping.MappingInfo

	// 使用成本计算函数获取旧成本价格（包含固定成本）
	oldCostPrice := s.getProductCostPrice(prod, skuMapping, storeID)
	if oldCostPrice <= 0 {
		s.logger.WithFields(logrus.Fields{
			"asin": mappingInfo.ProductId,
			"sku":  s.getStringValue(mappingInfo.Sku),
		}).Debug("无法获取TEMU旧成本价格，跳过利润率检查")
		return false
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(storeID)

	// 使用成本计算函数获取Amazon新成本价格（包含固定成本）
	newCostPrice := s.getAmazonProductCostPrice(amazonProduct, priceType, storeID)
	if newCostPrice <= 0 {
		s.logger.WithFields(logrus.Fields{
			"asin": mappingInfo.ProductId,
			"sku":  s.getStringValue(mappingInfo.Sku),
		}).Debug("无法获取Amazon新成本价格，跳过利润率检查")
		return false
	}

	// 获取TEMU产品的销售价格
	temuSalePrice := s.parsePrice(prod.OriginalPrice.String())
	if temuSalePrice <= 0 {
		temuSalePrice = s.parsePrice(prod.SpecialPrice.String())
	}

	if temuSalePrice <= 0 {
		s.logger.WithFields(logrus.Fields{
			"asin": mappingInfo.ProductId,
			"sku":  s.getStringValue(mappingInfo.Sku),
		}).Debug("无法获取TEMU销售价格，跳过利润率检查")
		return false
	}

	// 计算当前利润率和新利润率
	// 使用标准利润率公式：(售价 - 成本) / 成本
	// 注意：如果成本为0，跳过计算避免除零错误
	var oldProfitRate, newProfitRate float64

	if oldCostPrice > 0 {
		oldProfitRate = (temuSalePrice - oldCostPrice) / oldCostPrice
	}

	if newCostPrice > 0 {
		newProfitRate = (temuSalePrice - newCostPrice) / newCostPrice
	}

	// 获取最低利润率阈值
	minProfitRate := s.getMinProfitRateThreshold(storeID)

	// 判断是否需要处理价格变化
	needHandle := false
	reason := ""

	// 如果新利润率低于最低阈值，需要处理
	if newProfitRate < minProfitRate {
		needHandle = true
		reason = "新利润率低于最低阈值"
	}

	// 如果旧利润率低于阈值但新利润率高于阈值，也需要处理（恢复库存）
	if oldProfitRate < minProfitRate && newProfitRate >= minProfitRate {
		needHandle = true
		reason = "利润率从低于阈值恢复到高于阈值"
	}

	if needHandle {
		s.logger.WithFields(logrus.Fields{
			"asin":            mappingInfo.ProductId,
			"sku":             s.getStringValue(mappingInfo.Sku),
			"old_cost_price":  oldCostPrice,
			"new_cost_price":  newCostPrice,
			"temu_sale_price": temuSalePrice,
			"old_profit_rate": oldProfitRate,
			"new_profit_rate": newProfitRate,
			"min_profit_rate": minProfitRate,
			"reason":          reason,
		}).Info("检测到TEMU需要处理的价格变化")
		return true
	}

	return false
}

// checkStockChange 检查库存变化
func (s *inventorySyncServiceImpl) checkStockChange(
	amazonProduct *model.Product,
	skuMapping *TemuSkuInfo,
	storeID int64,
) bool {
	oldStock := skuMapping.UsableInventory
	newStock := s.extractStockFromProduct(amazonProduct)
	changeAmount := newStock - oldStock

	// 获取库存变化阈值（优先使用店铺级策略）
	threshold := s.getStockChangeThreshold(storeID)

	if s.absInt(changeAmount) >= threshold {
		s.logger.WithFields(logrus.Fields{
			"asin":          skuMapping.MappingInfo.ProductId,
			"sku":           s.getStringValue(skuMapping.MappingInfo.Sku),
			"old_stock":     oldStock,
			"new_stock":     newStock,
			"change_amount": changeAmount,
			"threshold":     threshold,
		}).Info("检测到TEMU库存变化")
		return true
	}
	return false
}

// getPreviousPrice 获取之前的价格（用于策略处理）
func (s *inventorySyncServiceImpl) getPreviousPrice(attributesJSON string, platformSKU string) float64 {
	if attributesJSON == "" {
		return 0
	}

	// 解析 Attributes 获取SKU映射列表
	skuMappingList := s.extractMappingInfoFromAttributes(attributesJSON)
	if len(skuMappingList) == 0 {
		return 0
	}

	// 查找对应的SKU
	for _, skuMapping := range skuMappingList {
		if s.getStringValue(skuMapping.SkuInfo[0].MappingInfo.Sku) == platformSKU {
			// TEMU结构中没有AmazonMonitorData，返回0表示没有历史价格
			return 0
		}
	}

	return 0
}

