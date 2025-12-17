// Package scheduler 提供同步调度器实现
package scheduler

import (
	"task-processor/internal/common/management"
)

// SyncScheduler 产品同步调度器（临时占位符）
type SyncScheduler struct {
	managementClient *management.ClientManager
}

// NewSyncScheduler 创建同步调度器
func NewSyncScheduler(managementClient *management.ClientManager) *SyncScheduler {
	return &SyncScheduler{
		managementClient: managementClient,
	}
}

// Start 启动调度器
func (s *SyncScheduler) Start() error {
	// TODO: 实现同步调度逻辑
	return nil
}

// Stop 停止调度器
func (s *SyncScheduler) Stop() {
	// TODO: 实现停止逻辑
}

// RegisterSheinStore 注册SHEIN店铺
func (s *SyncScheduler) RegisterSheinStore(storeID int64, client interface{}) {
	// TODO: 实现SHEIN店铺注册逻辑
}

// RegisterTemuStore 注册TEMU店铺
func (s *SyncScheduler) RegisterTemuStore(storeID int64, client interface{}) {
	// TODO: 实现TEMU店铺注册逻辑
}
