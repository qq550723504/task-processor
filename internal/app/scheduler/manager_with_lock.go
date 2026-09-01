// Package scheduler 提供任务调度功能
package scheduler

import (
	"context"
	"time"
)

// ManagerWithLock 带分布式锁的调度器管理器
type ManagerWithLock struct {
	*Manager
	distributedLock DistributedLock
	enableLock      bool
}

// NewManagerWithLock 创建带分布式锁的调度器管理器
func NewManagerWithLock(ctx context.Context, taskTimeout time.Duration, distributedLock DistributedLock, enableLock bool) *ManagerWithLock {
	baseManager := NewManager(ctx, taskTimeout)
	if enableLock && distributedLock != nil {
		baseManager.SetDistributedLock(distributedLock, taskTimeout)
	}

	return &ManagerWithLock{
		Manager:         baseManager,
		distributedLock: distributedLock,
		enableLock:      enableLock,
	}
}

// CreateAndStartTask 创建并启动任务（支持分布式锁）
func (m *ManagerWithLock) CreateAndStartTask(config TaskConfig) error {
	if m.enableLock && m.distributedLock != nil {
		m.Manager.SetDistributedLock(m.distributedLock, m.taskTimeout)
	}
	return m.Manager.CreateAndStartTask(config)
}

// GetDistributedLock 获取分布式锁实例
func (m *ManagerWithLock) GetDistributedLock() DistributedLock {
	return m.distributedLock
}

// IsLockEnabled 检查是否启用了分布式锁
func (m *ManagerWithLock) IsLockEnabled() bool {
	return m.enableLock
}
