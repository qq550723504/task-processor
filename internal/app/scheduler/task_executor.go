// Package scheduler 提供任务执行器功能
package scheduler

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"task-processor/internal/core/logger"
	infralock "task-processor/internal/infra/lock"
	"task-processor/internal/pkg/timeout"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskExecutor 任务执行器
type TaskExecutor struct {
	task              Task
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	taskTimeout       time.Duration
	logger            *logrus.Entry
	dependencyManager *DependencyManager
	isRunning         int32          // 任务执行状态标志 (0: 空闲, 1: 执行中)
	started           int32          // 执行器启动状态标志 (0: 未启动, 1: 已启动)
	skipCount         int64          // 跳过执行次数统计
	stats             *ExecutorStats // 执行统计
	distributedLock   infralock.DistributedLock
	lockTTL           time.Duration
}

// NewTaskExecutor 创建新的任务执行器
func NewTaskExecutor(ctx context.Context, task Task, depManager *DependencyManager, taskTimeout time.Duration) *TaskExecutor {
	executorCtx, cancel := context.WithCancel(ctx)
	if taskTimeout <= 0 {
		taskTimeout = timeout.TaskExtraTimeout
	}

	return &TaskExecutor{
		task:              task,
		ctx:               executorCtx,
		cancel:            cancel,
		taskTimeout:       taskTimeout,
		dependencyManager: depManager,
		stats:             NewExecutorStats(),
		logger: logger.GetGlobalLogger("task_executor").WithFields(logrus.Fields{
			logger.FieldComponent: "task_executor",
			"task_id":             task.GetID(),
			"task_type":           task.GetType(),
			logger.FieldPlatform:  task.GetPlatform(),
		}),
	}
}

// SetDistributedLock 为任务执行器设置分布式锁。
func (e *TaskExecutor) SetDistributedLock(locker infralock.DistributedLock, ttl time.Duration) {
	e.distributedLock = locker
	if ttl <= 0 {
		ttl = e.taskTimeout
	}
	e.lockTTL = ttl
}

// Start 启动任务执行器
func (e *TaskExecutor) Start() bool {
	if !atomic.CompareAndSwapInt32(&e.started, 0, 1) {
		e.logger.Info("任务执行器已启动，跳过重复启动")
		return false
	}
	e.logger.WithField("interval", e.task.GetInterval()).Info("启动任务执行器")

	e.wg.Add(1)
	go e.run()
	return true
}

// Stop 停止任务执行器
func (e *TaskExecutor) Stop() {
	e.logger.Info("停止任务执行器")
	e.cancel()
	e.wg.Wait()
	e.logger.Info("任务执行器已停止")
}

// run 运行任务执行器
func (e *TaskExecutor) run() {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			e.logger.WithField("panic", r).Error("任务执行器发生panic")
		}
	}()

	// 立即执行一次
	e.executeTaskWithConcurrencyControl()

	ticker := time.NewTicker(e.task.GetInterval())
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("收到停止信号，退出任务执行器")
			return
		case <-ticker.C:
			e.executeTaskWithConcurrencyControl()
		}
	}
}

// executeTaskWithConcurrencyControl 带并发控制的任务执行
func (e *TaskExecutor) executeTaskWithConcurrencyControl() {
	// 使用原子操作检查并设置执行状态
	if !atomic.CompareAndSwapInt32(&e.isRunning, 0, 1) {
		// 任务正在执行中，跳过本次执行
		skipCount := atomic.AddInt64(&e.skipCount, 1)
		e.stats.RecordSkip()

		e.logger.WithField("skip_count", skipCount).Warn("上一个任务还在执行中，跳过本次执行")
		return
	}

	// 执行完成后重置状态
	defer atomic.StoreInt32(&e.isRunning, 0)

	if e.distributedLock != nil {
		key := e.distributedLockKey()
		locked, err := e.distributedLock.TryLock(e.ctx, key, e.lockTTL)
		if err != nil {
			e.logger.WithError(err).WithField("lock_key", key).Error("获取任务分布式锁失败")
			e.stats.RecordSkip()
			atomic.AddInt64(&e.skipCount, 1)
			return
		}
		if !locked {
			e.logger.WithField("lock_key", key).Info("任务分布式锁已被其他实例持有，跳过本次执行")
			e.stats.RecordSkip()
			atomic.AddInt64(&e.skipCount, 1)
			return
		}
		defer func() {
			if err := e.distributedLock.Unlock(context.Background(), key); err != nil {
				e.logger.WithError(err).WithField("lock_key", key).Warn("释放任务分布式锁失败")
			}
		}()
	}

	// 执行实际任务
	e.executeTask()
}

func (e *TaskExecutor) distributedLockKey() string {
	return fmt.Sprintf("listing:scheduler:lock:%s:%s:%d", e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID())
}

// executeTask 执行任务
func (e *TaskExecutor) executeTask() {
	defer func() {
		if r := recover(); r != nil {
			// 获取堆栈信息
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			e.logger.WithFields(logrus.Fields{
				"panic": r,
				"stack": string(buf[:n]),
			}).Error("执行任务时发生panic")
		}
	}()

	startTime := time.Now()
	e.logger.Info("开始执行任务")

	// 创建任务上下文，设置超时
	taskCtx, cancel := context.WithTimeout(e.ctx, e.taskTimeout)
	defer cancel()

	// 检查依赖任务是否满足
	if e.dependencyManager != nil {
		err := e.dependencyManager.WaitForDependencies(taskCtx, e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID())
		if err != nil {
			e.logger.WithError(err).Warn("依赖任务未满足,跳过本次执行")
			e.dependencyManager.UpdateTaskStatus(e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID(), "skipped", err)
			return
		}
	}

	// 更新任务状态为运行中
	if e.dependencyManager != nil {
		e.dependencyManager.UpdateTaskStatus(e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID(), "running", nil)
	}

	// 执行任务
	err := e.task.Execute(taskCtx)

	duration := time.Since(startTime)
	success := err == nil

	// 记录执行统计
	e.stats.RecordExecution(duration, success)

	// 更新任务执行状态
	if e.dependencyManager != nil {
		if err != nil {
			e.dependencyManager.UpdateTaskStatus(e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID(), "failed", err)
		} else {
			e.dependencyManager.UpdateTaskStatus(e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID(), "success", nil)
		}
	}

	if err != nil {
		e.logger.WithError(err).WithField(logger.FieldDurationMs, duration.Milliseconds()).Error("任务执行失败")
	} else {
		e.logger.WithField(logger.FieldDurationMs, duration.Milliseconds()).Info("任务执行成功")
	}
}

// GetTask 获取任务
func (e *TaskExecutor) GetTask() Task {
	return e.task
}

// IsRunning 检查任务是否正在执行
func (e *TaskExecutor) IsRunning() bool {
	return atomic.LoadInt32(&e.isRunning) == 1
}

// GetSkipCount 获取跳过执行次数
func (e *TaskExecutor) GetSkipCount() int64 {
	return atomic.LoadInt64(&e.skipCount)
}

// ResetSkipCount 重置跳过执行次数统计
func (e *TaskExecutor) ResetSkipCount() {
	atomic.StoreInt64(&e.skipCount, 0)
	e.logger.Info("已重置跳过执行次数统计")
}

// GetStats 获取执行统计信息
func (e *TaskExecutor) GetStats() ExecutorStatsSnapshot {
	return e.stats.GetStats()
}

// ResetStats 重置统计信息
func (e *TaskExecutor) ResetStats() {
	e.stats.Reset()
	e.ResetSkipCount()
	e.logger.Info("已重置所有统计信息")
}
