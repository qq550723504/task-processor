// Package shein 提供成本价和利润率计算相关方法
package shein

import (
	"task-processor/internal/common/management/api"
	"task-processor/internal/platforms/shein/client/api/marketing"
)

// CostProfitCalculator 成本利润计算器
type CostProfitCalculator struct{}

// NewCostProfitCalculator 创建成本利润计算器
func NewCostProfitCalculator() *CostProfitCalculator {
	return &CostProfitCalculator{}
}

// CalculateCostAndProfit 计算成本价和利润率
func (c *CostProfitCalculator) CalculateCostAndProfit(product marketing.SkcInfo) (costPrice, profitRate float64) {
	// 基于供货价格计算成本价（假设成本价为供货价格的85%）
	costPrice = product.SupplyPrice * 0.85

	// 计算平均销售价格
	avgSalePrice := c.calculateAverageSalePrice(product.SitePriceInfoList)

	// 计算利润率：(销售价格 - 成本价) / 成本价 * 100
	if costPrice > 0 && avgSalePrice > costPrice {
		profitRate = ((avgSalePrice - costPrice) / costPrice) * 100
	}

	return costPrice, profitRate
}

// calculateAverageSalePrice 计算平均销售价格
func (c *CostProfitCalculator) calculateAverageSalePrice(sitePrices []marketing.SitePriceInfo) float64 {
	if len(sitePrices) == 0 {
		return 0
	}

	var totalPrice float64
	var validPriceCount int

	for _, sitePrice := range sitePrices {
		if sitePrice.IsAvailable && sitePrice.SalePrice > 0 {
			totalPrice += sitePrice.SalePrice
			validPriceCount++
		}
	}

	if validPriceCount == 0 {
		return 0
	}

	return totalPrice / float64(validPriceCount)
}

// EnrichActivityProductWithCostProfit 为活动产品添加成本利润信息
func (c *CostProfitCalculator) EnrichActivityProductWithCostProfit(product *api.ActivityProductDTO, skcInfo marketing.SkcInfo) {
	costPrice, profitRate := c.CalculateCostAndProfit(skcInfo)
	product.CostPrice = costPrice
	product.ProfitRate = profitRate
}

// EnrichRegistrationWithCostProfit 为报名记录添加成本利润信息
func (c *CostProfitCalculator) EnrichRegistrationWithCostProfit(registration *api.ActivityRegistrationDTO, skcInfo marketing.SkcInfo) {
	costPrice, profitRate := c.CalculateCostAndProfit(skcInfo)
	registration.CostPrice = costPrice
	registration.ProfitRate = profitRate
}
