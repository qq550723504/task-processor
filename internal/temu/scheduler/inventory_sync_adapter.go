// Package scheduler 提供TEMU库存同步任务的适配器
package scheduler

import (
	"context"

	managementapi "task-processor/internal/infra/clients/management/api"
	platformtask "task-processor/internal/platformtask"
	temuscheduler "task-processor/internal/temu/sync"
)

// inventorySyncServiceAdapter 适配器，将TEMU特定的InventorySyncService适配到通用接口
type inventorySyncServiceAdapter struct {
	temuService temuscheduler.InventorySyncService
}

// newInventorySyncServiceAdapter 创建库存同步服务适配器
func newInventorySyncServiceAdapter(temuService temuscheduler.InventorySyncService) platformtask.InventorySyncService {
	return &inventorySyncServiceAdapter{
		temuService: temuService,
	}
}

// FetchProductsForInventorySync 获取需要监控库存的产品列表（适配到通用接口）
func (a *inventorySyncServiceAdapter) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]any, error) {
	products, err := a.temuService.FetchProductsForInventorySync(ctx, tenantID, storeID)
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
func (a *inventorySyncServiceAdapter) MonitorInventoryChanges(ctx context.Context, products []any, tenantID, storeID int64) (*platformtask.InventorySyncResult, error) {
	// 转换回TEMU特定类型
	temuProducts := make([]*managementapi.ProductDataDTO, len(products))
	for i, p := range products {
		if tp, ok := p.(*managementapi.ProductDataDTO); ok {
			temuProducts[i] = tp
		}
	}

	// 调用TEMU服务
	result, err := a.temuService.MonitorInventoryChanges(ctx, temuProducts, tenantID, storeID)
	if err != nil {
		return nil, err
	}

	// 转换为通用结果类型
	return &platformtask.InventorySyncResult{
		TotalProducts:     result.TotalProducts,
		ProcessedProducts: result.ProcessedProducts,
		SkippedProducts:   result.SkippedProducts,
		PriceChanges:      result.PriceChanges,
		StockChanges:      result.StockChanges,
		AmazonFetched:     result.AmazonFetched,
		AmazonFailed:      result.AmazonFailed,
	}, nil
}
