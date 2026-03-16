// Package taskexecutor 提供SHEIN库存同步任务的适配器
package taskexecutor

import (
	"context"

	managementapi "task-processor/internal/infra/clients/management/api"
	commonscheduler "task-processor/internal/taskbase"
	sheinscheduler "task-processor/internal/shein/operation"
)

// inventorySyncServiceAdapter 适配器，将SHEIN特定的InventorySyncService适配到通用接口
type inventorySyncServiceAdapter struct {
	sheinService sheinscheduler.InventorySyncService
}

// newInventorySyncServiceAdapter 创建库存同步服务适配器
func newInventorySyncServiceAdapter(sheinService sheinscheduler.InventorySyncService) commonscheduler.InventorySyncService {
	return &inventorySyncServiceAdapter{
		sheinService: sheinService,
	}
}

// FetchProductsForInventorySync 获取需要监控库存的产品列表（适配到通用接口）
func (a *inventorySyncServiceAdapter) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]any, error) {
	products, err := a.sheinService.FetchProductsForInventorySync(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]any, len(products))
	for i, p := range products {
		result[i] = p
	}
	return result, nil
}

// MonitorInventoryChanges 监控库存和价格变化（适配到通用接口）
func (a *inventorySyncServiceAdapter) MonitorInventoryChanges(ctx context.Context, products []any, tenantID, storeID int64) (*commonscheduler.InventorySyncResult, error) {
	// 转换回SHEIN特定类型
	sheinProducts := make([]*managementapi.ProductDataDTO, len(products))
	for i, p := range products {
		if sp, ok := p.(*managementapi.ProductDataDTO); ok {
			sheinProducts[i] = sp
		}
	}

	// 调用SHEIN服务
	result, err := a.sheinService.MonitorInventoryChanges(ctx, sheinProducts, tenantID, storeID)
	if err != nil {
		return nil, err
	}

	// 转换为通用结果类型
	return &commonscheduler.InventorySyncResult{
		TotalProducts:     result.TotalProducts,
		ProcessedProducts: result.ProcessedProducts,
		SkippedProducts:   result.SkippedProducts,
		PriceChanges:      result.PriceChanges,
		StockChanges:      result.StockChanges,
		AmazonFetched:     result.AmazonFetched,
		AmazonFailed:      result.AmazonFailed,
	}, nil
}
