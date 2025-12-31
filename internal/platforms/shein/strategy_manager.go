// Package shein 提供SHEIN运营策略管理功能
package shein

import (
	shops "task-processor/internal/common/shein"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// StrategyManager 策略管理器
type StrategyManager struct {
	operationStrategyClient api.OperationStrategyAPI
}

// NewStrategyManager 创建策略管理器
func NewStrategyManager(operationStrategyClient api.OperationStrategyAPI) *StrategyManager {
	return &StrategyManager{
		operationStrategyClient: operationStrategyClient,
	}
}

// ExecuteStrategy 执行运营策略
func (m *StrategyManager) ExecuteStrategy(
	strategy *api.OperationStrategyDTO,
	apiClient *shops.ShopAPIClient,
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
	logger *logrus.Entry,
) int {
	executor := NewStrategyExecutor(strategy, apiClient)
	executedCount := 0

	// 策略执行优先级：缺货 > 库存变化 > 低利润率
	// 使用标志位避免重复执行相同操作
	executed := false

	// 1. 优先执行缺货策略（最高优先级）
	if !amazonProduct.IsAvailable {
		if err := executor.ExecuteOutOfStock(prod, skuMapping, amazonProduct); err != nil {
			logger.WithError(err).Warn("执行缺货策略失败")
		} else if strategy.OutOfStockAction != "NONE" {
			executedCount++
			executed = true
			logger.Debug("已执行缺货策略，跳过库存变化策略")
		}
	}

	// 2. 执行库存变化策略（仅当未执行缺货策略时）
	if !executed {
		if err := executor.ExecuteStockChange(prod, skuMapping, amazonProduct); err != nil {
			logger.WithError(err).Warn("执行库存变化策略失败")
		} else if strategy.StockChangeAction != "NONE" {
			executedCount++
		}
	}

	// 3. 执行低利润率策略（独立执行，不受其他策略影响）
	if err := executor.ExecuteLowProfit(prod, skuMapping, amazonProduct); err != nil {
		logger.WithError(err).Warn("执行低利润率策略失败")
	} else if strategy.LowProfitAction != "NONE" {
		executedCount++
	}

	return executedCount
}
