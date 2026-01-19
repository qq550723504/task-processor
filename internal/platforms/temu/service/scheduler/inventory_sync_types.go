// Package scheduler 提供TEMU库存同步相关类型定义
package scheduler

import (
	"context"
	"task-processor/internal/platforms/shein/api/product"
)

// InventorySyncService TEMU库存同步服务接口
type InventorySyncService interface {
	// SyncInventory 同步库存
	SyncInventory(ctx context.Context, productIDs []string) error

	// GetInventoryStatus 获取库存状态
	GetInventoryStatus(ctx context.Context, productID string) (*product.InventoryInfo, error)

	// UpdateInventoryBatch 批量更新库存
	UpdateInventoryBatch(ctx context.Context, updates []InventoryUpdate) error
}

// InventoryUpdate 库存更新信息
type InventoryUpdate struct {
	ProductID string `json:"product_id"`
	Stock     int    `json:"stock"`
	Operation string `json:"operation"` // "set", "add", "subtract"
}

// InventorySyncConfig 库存同步配置
type InventorySyncConfig struct {
	BatchSize      int  `json:"batch_size"`
	EnableAutoSync bool `json:"enable_auto_sync"`
	SyncInterval   int  `json:"sync_interval"` // 秒
}
