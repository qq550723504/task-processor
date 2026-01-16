// Package scheduler 提供任务执行器功能
package scheduler

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskExecutor 任务执行器
type TaskExecutor struct {
	task              Task
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	logger            *logrus.Entry
	dependencyManager *DependencyManager
}

// NewTaskExecutor 创建新的任务执行器
func NewTaskExecutor(ctx context.Context, task Task, depManager *DependencyManager) *TaskExecutor {
	executorCtx, cancel := context.WithCancel(ctx)

	return &TaskExecutor{
		task:              task,
		ctx:               executorCtx,
		cancel:            cancel,
		dependencyManager: depManager,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TaskExecutor",
			"task_id":   task.GetID(),
			"task_type": task.GetType(),
			"platform":  task.GetPlatform(),
		}),
	}
}

// Start 启动任务执行器
func (e *TaskExecutor) Start() {
	e.logger.Infof("启动任务执行器，间隔: %v", e.task.GetInterval())

	e.wg.Add(1)
	go e.run()
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
			e.logger.Errorf("任务执行器发生panic: %v", r)
		}
	}()

	// 立即执行一次
	e.executeTask()

	ticker := time.NewTicker(e.task.GetInterval())
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("收到停止信号，退出任务执行器")
			return
		case <-ticker.C:
			e.executeTask()
		}
	}
}

// executeTask 执行任务
func (e *TaskExecutor) executeTask() {
	defer func() {
		if r := recover(); r != nil {
			// 获取堆栈信息
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			e.logger.Errorf("执行任务时发生panic: %v\n堆栈:\n%s", r, string(buf[:n]))
		}
	}()

	startTime := time.Now()
	e.logger.Info("开始执行任务")

	// 创建任务上下文，设置超时
	taskCtx, cancel := context.WithTimeout(e.ctx, 30*time.Minute)
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

	// 更新任务执行状态
	if e.dependencyManager != nil {
		if err != nil {
			e.dependencyManager.UpdateTaskStatus(e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID(), "failed", err)
		} else {
			e.dependencyManager.UpdateTaskStatus(e.task.GetPlatform(), e.task.GetType(), e.task.GetStoreID(), "success", nil)
		}
	}

	if err != nil {
		e.logger.WithError(err).Errorf("任务执行失败，耗时: %v", duration)
	} else {
		e.logger.Infof("任务执行成功，耗时: %v", duration)
	}
}

// GetTask 获取任务
func (e *TaskExecutor) GetTask() Task {
	return e.task
}
