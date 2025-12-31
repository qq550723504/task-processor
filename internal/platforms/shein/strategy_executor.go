// Package shein 提供SHEIN平台的运营策略执行功能
package shein

import (
	"task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/domain/model"
)

// StrategyExecutor 运营策略执行器
type StrategyExecutor struct {
	strategy       *api.OperationStrategyDTO
	apiClient      *shops.ShopAPIClient
	stockManager   *StockStrategyManager
	profitManager  *ProfitStrategyManager
	shelfManager   *ShelfOperationManager
	requestBuilder *StrategyRequestBuilder
}

// NewStrategyExecutor 创建策略执行器
func NewStrategyExecutor(strategy *api.OperationStrategyDTO, apiClient *shops.ShopAPIClient) *StrategyExecutor {
	return &StrategyExecutor{
		strategy:       strategy,
		apiClient:      apiClient,
		stockManager:   NewStockStrategyManager(strategy, apiClient),
		profitManager:  NewProfitStrategyManager(strategy, apiClient),
		shelfManager:   NewShelfOperationManager(apiClient),
		requestBuilder: NewStrategyRequestBuilder(),
	}
}

// ExecuteStockChange 执行库存变化策略
// 返回 error 表示执行失败，返回 nil 表示执行成功或不需要执行
func (e *StrategyExecutor) ExecuteStockChange(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
) error {
	return e.stockManager.ExecuteStockChange(prod, skuMapping, amazonProduct)
}

// ExecuteOutOfStock 执行缺货策略
// 返回 error 表示执行失败，返回 nil 表示执行成功或不需要执行
func (e *StrategyExecutor) ExecuteOutOfStock(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
) error {
	return e.stockManager.ExecuteOutOfStock(prod, skuMapping, amazonProduct)
}

// ExecuteLowProfit 执行低利润率策略
// 返回 error 表示执行失败，返回 nil 表示执行成功或不需要执行
func (e *StrategyExecutor) ExecuteLowProfit(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
) error {
	return e.profitManager.ExecuteLowProfit(prod, skuMapping, amazonProduct)
}
