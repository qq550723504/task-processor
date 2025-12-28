// Package service 提供活动数据丰富化服务
package service

import (
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/shein/api/marketing"
)

// ActivityEnricher 活动数据丰富化器
type ActivityEnricher struct {
	calculator *CostProfitCalculator
}

// NewActivityEnricher 创建活动数据丰富化器
func NewActivityEnricher() *ActivityEnricher {
	return &ActivityEnricher{
		calculator: NewCostProfitCalculator(),
	}
}

// EnrichActivityProductWithCostProfit 为活动产品添加成本利润信息
func (e *ActivityEnricher) EnrichActivityProductWithCostProfit(product *api.ActivityProductDTO, skcInfo marketing.SkcInfo) {
	costPrice, profitRate := e.calculator.CalculateCostAndProfit(skcInfo)
	product.CostPrice = costPrice
	product.ProfitRate = profitRate
}

// EnrichRegistrationWithCostProfit 为报名记录添加成本利润信息
func (e *ActivityEnricher) EnrichRegistrationWithCostProfit(registration *api.ActivityRegistrationDTO, skcInfo marketing.SkcInfo) {
	costPrice, profitRate := e.calculator.CalculateCostAndProfit(skcInfo)
	registration.CostPrice = costPrice
	registration.ProfitRate = profitRate
}
