// Package shein 提供SHEIN记录管理功能
package shein

import (
	"task-processor/internal/common/management/api"
)

// RecordManager 记录管理器
type RecordManager struct {
	inventoryRecordClient api.InventoryRecordAPI
}

// NewRecordManager 创建记录管理器
func NewRecordManager(inventoryRecordClient api.InventoryRecordAPI) *RecordManager {
	return &RecordManager{
		inventoryRecordClient: inventoryRecordClient,
	}
}
