// Package shein 提供成本价和利润率计算相关方法
package shein

import (
	"task-processor/internal/common/shein/api/marketing"
	"task-processor/internal/common/shein/service"
	"task-processor/internal/pkg/management/api"
)

// CostProfitCalculator 成本利润计算器（适配器模式）
type CostProfitCalculator struct {
	calculator *service.CostProfitCalculator
}

// NewCostProfitCalculator 创建成本利润计算器
func NewCostProfitCalculator() *CostProfitCalculator {
	return &CostProfitCalculator{
		calculator: service.NewCostProfitCalculator(),
	}
}

// CalculateCostAndProfit 计算成本价和利润率
func (c *CostProfitCalculator) CalculateCostAndProfit(product marketing.SkcInfo) (costPrice, profitRate float64) {
	return c.calculator.CalculateCostAndProfit(product)
}

// EnrichActivityProductWithCostProfit 为活动产品添加成本利润信息
func (c *CostProfitCalculator) EnrichActivityProductWithCostProfit(product *api.ActivityProductDTO, skcInfo marketing.SkcInfo) {
	enricher := service.NewActivityEnricher()
	enricher.EnrichActivityProductWithCostProfit(product, skcInfo)
}

// EnrichRegistrationWithCostProfit 为报名记录添加成本利润信息
func (c *CostProfitCalculator) EnrichRegistrationWithCostProfit(registration *api.ActivityRegistrationDTO, skcInfo marketing.SkcInfo) {
	enricher := service.NewActivityEnricher()
	enricher.EnrichRegistrationWithCostProfit(registration, skcInfo)
}
