// package sync 提供TEMU库存同步相关类型定义
package sync

import (
	"context"
)

// InventorySyncService TEMU库存监控服务接口（监控Amazon价格和库存变化）
type InventorySyncService interface {
	// FetchProductsForInventorySync 获取需要监控库存的产品列表
	FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*TemuInventoryProductSnapshot, error)

	// MonitorInventoryChanges 监控库存和价格变化
	MonitorInventoryChanges(ctx context.Context, products []*TemuInventoryProductSnapshot, tenantID, storeID int64) (*MonitorResult, error)
}
