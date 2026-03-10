// Package scheduler 提供TEMU库存同步任务的适配器
package scheduler

import (
	"context"

	managementapi "task-processor/internal/pkg/management/api"
	commonscheduler "task-processor/internal/platforms/common/scheduler"
	temuscheduler "task-processor/internal/platforms/temu/services/business_service"
)

// inventorySyncServiceAdapter 适配器，将TEMU特定的InventorySyncService适配到通用接口
type inventorySyncServiceAdapter struct {
	temuService temuscheduler.InventorySyncService
}

// newInventorySyncServiceAdapter 创建库存同步服务适配器
func newInventorySyncServiceAdapter(temuService temuscheduler.InventorySyncService) commonscheduler.InventorySyncService {
	return &inventorySyncServiceAdapter{
		temuService: temuService,
	}
}

// FetchProductsForInventorySync 获取需要监控库存的产品列表（适配到通用接口）
func (a *inventorySyncServiceAdapter) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]interface{}, error) {
	products, err := a.temuService.FetchProductsForInventorySync(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]interface{}, len(products))
	for i, p := range products {
		result[i] = p
	}
	return result, nil
}

// MonitorInventoryChanges 监控库存和价格变化（适配到通用接口）
func (a *inventorySyncServiceAdapter) MonitorInventoryChanges(ctx context.Context, products []interface{}, tenantID, storeID int64) (*commonscheduler.InventorySyncResult, error) {
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
