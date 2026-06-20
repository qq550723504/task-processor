// package sync 提供TEMU平台调度器相关服务
package sync

import (
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// getProductCostPrice 获取产品成本价格（包含固定成本）
func (s *inventorySyncServiceImpl) getProductCostPrice(
	prod *TemuInventoryProductSnapshot,
	skuMapping *TemuSkuInfo,
	storeID int64,
) float64 {
	mappingInfo := skuMapping.MappingInfo

	// 获取基础成本价格
	baseCostPrice := s.getFloatValue(mappingInfo.CostPrice)
	if baseCostPrice <= 0 {
		baseCostPrice = s.parsePrice(prod.OriginalPrice.String())
		if baseCostPrice <= 0 {
			baseCostPrice = s.parsePrice(prod.SpecialPrice.String())
		}
	}

	if baseCostPrice <= 0 {
		s.logger.WithFields(logrus.Fields{
			"product_id": prod.ProductID,
			"sku":        s.getStringValue(mappingInfo.Sku),
		}).Debug("无法获取基础成本价格")
		return 0
	}

	// 使用通用成本计算器
	return s.costCalculator.CalculateProductCost(
		baseCostPrice,
		storeID,
		prod.ProductID,
		s.getStringValue(mappingInfo.Sku),
	)
}

// getAmazonProductCostPrice 获取Amazon产品成本价格（包含固定成本）
func (s *inventorySyncServiceImpl) getAmazonProductCostPrice(
	amazonProduct *model.Product,
	priceType string,
	storeID int64,
) float64 {
	// 使用通用成本计算器
	return s.costCalculator.CalculateAmazonProductCost(amazonProduct, priceType, storeID)
}
