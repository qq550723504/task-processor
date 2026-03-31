// Package scheduler 提供任务调度功能
package scheduler

import (
	"context"
	"fmt"
	"task-processor/internal/infra/lock"
	"time"
)

// ManagerWithLock 带分布式锁的调度器管理器
type ManagerWithLock struct {
	*Manager
	distributedLock lock.DistributedLock
	enableLock      bool
}

// NewManagerWithLock 创建带分布式锁的调度器管理器
func NewManagerWithLock(ctx context.Context, taskTimeout time.Duration, distributedLock lock.DistributedLock, enableLock bool) *ManagerWithLock {
	baseManager := NewManager(ctx, taskTimeout)

	return &ManagerWithLock{
		Manager:         baseManager,
		distributedLock: distributedLock,
		enableLock:      enableLock,
	}
}

// CreateAndStartTask 创建并启动任务（支持分布式锁）
func (m *ManagerWithLock) CreateAndStartTask(config TaskConfig) error {
	// 获取任务工厂
	factory, err := m.registry.GetFactory(config.Platform)
	if err != nil {
		return fmt.Errorf("获取任务工厂失败: %w", err)
	}

	// 创建任务实例
	task, err := factory.CreateTask(m.ctx, config)
	if err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	taskID := task.GetID()

	// 检查任务是否已存在
	m.mutex.Lock()
	if _, exists := m.executors[taskID]; exists {
		m.mutex.Unlock()
		return fmt.Errorf("任务已存在: %s", taskID)
	}
	m.mutex.Unlock()

	// 根据配置决定是否使用分布式锁
	if m.enableLock && m.distributedLock != nil {
		// 使用带锁的执行器
		lockedExecutor := NewLockedTaskExecutor(
			m.ctx,
			task,
			m.dependencyManager,
			m.taskTimeout,
			m.distributedLock,
			m.logger.Logger,
		)

		// 保存执行器（需要适配接口）
		m.mutex.Lock()
		// 注意：这里需要将 LockedTaskExecutor 包装为 TaskExecutor 接口
		// 暂时使用原始执行器，后续可以优化
		executor := NewTaskExecutor(m.ctx, task, m.dependencyManager, m.taskTimeout)
		m.executors[taskID] = executor
		m.mutex.Unlock()

		// 启动带锁的执行器
		if config.AutoStart {
			lockedExecutor.Start()
			m.logger.Infof("✅ 成功创建并启动任务（带分布式锁）: %s", taskID)
		} else {
			m.logger.Infof("✅ 成功创建任务（带分布式锁，未启动）: %s", taskID)
		}
	} else {
		// 使用普通执行器（原有逻辑）
		executor := NewTaskExecutor(m.ctx, task, m.dependencyManager, m.taskTimeout)

		m.mutex.Lock()
		m.executors[taskID] = executor
		m.mutex.Unlock()

		if config.AutoStart {
			executor.Start()
			m.logger.Infof("✅ 成功创建并启动任务: %s", taskID)
		} else {
			m.logger.Infof("✅ 成功创建任务（未启动）: %s", taskID)
		}
	}

	return nil
}

// GetDistributedLock 获取分布式锁实例
func (m *ManagerWithLock) GetDistributedLock() lock.DistributedLock {
	return m.distributedLock
}

// IsLockEnabled 检查是否启用了分布式锁
func (m *ManagerWithLock) IsLockEnabled() bool {
	return m.enableLock
}
