// Package scheduler 提供任务调度功能
package scheduler

import (
	"context"
	"fmt"
	"task-processor/internal/infra/lock"
	"time"

	"github.com/sirupsen/logrus"
)

// LockedTaskExecutor 带分布式锁的任务执行器
// 用于防止多实例部署时任务重复执行
type LockedTaskExecutor struct {
	executor *TaskExecutor
	lock     lock.DistributedLock
	logger   *logrus.Logger
	lockKey  string
	lockTTL  time.Duration
}

// NewLockedTaskExecutor 创建带锁的任务执行器
func NewLockedTaskExecutor(
	ctx context.Context,
	task Task,
	depManager *DependencyManager,
	taskTimeout time.Duration,
	distributedLock lock.DistributedLock,
	logger *logrus.Logger,
) *LockedTaskExecutor {
	executor := NewTaskExecutor(ctx, task, depManager, taskTimeout)

	// 生成锁的键名：platform:taskType:storeID
	lockKey := fmt.Sprintf("task:lock:%s:%s", task.GetPlatform(), task.GetType())

	// 锁的TTL设置为任务间隔的2倍，确保任务执行完成前锁不会过期
	lockTTL := task.GetInterval() * 2
	if lockTTL < 30*time.Second {
		lockTTL = 30 * time.Second // 最小30秒
	}

	return &LockedTaskExecutor{
		executor: executor,
		lock:     distributedLock,
		logger:   logger,
		lockKey:  lockKey,
		lockTTL:  lockTTL,
	}
}

// Start 启动任务执行器
func (e *LockedTaskExecutor) Start() {
	e.logger.Infof("[LockedTaskExecutor] 启动带锁的任务执行器: %s", e.lockKey)
	go e.run()
}

// Stop 停止任务执行器
func (e *LockedTaskExecutor) Stop() {
	e.logger.Infof("[LockedTaskExecutor] 停止带锁的任务执行器: %s", e.lockKey)
	e.executor.Stop()
}

// run 执行循环
func (e *LockedTaskExecutor) run() {
	// 立即执行一次
	e.executeWithLock()

	// 创建定时器
	ticker := time.NewTicker(e.executor.task.GetInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.executeWithLock()
		case <-e.executor.ctx.Done():
			e.logger.Infof("[LockedTaskExecutor] 任务执行器已停止: %s", e.lockKey)
			return
		}
	}
}

// executeWithLock 使用分布式锁执行任务
func (e *LockedTaskExecutor) executeWithLock() {
	ctx := e.executor.ctx

	// 尝试获取锁
	acquired, err := e.lock.TryLock(ctx, e.lockKey, e.lockTTL)
	if err != nil {
		e.logger.Errorf("[LockedTaskExecutor] 获取锁失败: %v", err)
		e.executor.stats.RecordSkip()
		return
	}

	if !acquired {
		e.logger.Debugf("[LockedTaskExecutor] 锁已被其他实例持有，跳过执行: %s", e.lockKey)
		e.executor.stats.RecordSkip()
		return
	}

	// 确保执行完成后释放锁
	defer func() {
		if err := e.lock.Unlock(ctx, e.lockKey); err != nil {
			e.logger.Errorf("[LockedTaskExecutor] 释放锁失败: %v", err)
		}
	}()

	// 执行任务
	e.logger.Debugf("[LockedTaskExecutor] 获取锁成功，开始执行任务: %s", e.lockKey)
	e.executor.executeTaskWithConcurrencyControl()
}

// GetStats 获取执行统计
func (e *LockedTaskExecutor) GetStats() ExecutorStatsSnapshot {
	return e.executor.GetStats()
}

// GetTaskID 获取任务ID
func (e *LockedTaskExecutor) GetTaskID() string {
	return e.executor.task.GetID()
}
