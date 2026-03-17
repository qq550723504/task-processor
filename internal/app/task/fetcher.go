// Package task 提供任务获取核心功能
package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

// UnifiedTaskFetcher 统一任务获取器
type TaskFetcher struct {
	*lifecycle.BaseComponent
	config           *config.Config
	managementClient *management.ClientManager
	submitters       map[string]TaskSubmitter // platform -> submitter
	interval         time.Duration
	processingTasks  map[string]time.Time // taskID -> 提交时间，用于去重
	tasksMutex       sync.RWMutex         // 保护 processingTasks

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *logrus.Logger

	// 子服务
	cleanupService *CleanupService
	monitorService *MonitorService
}

// NewUnifiedTaskFetcher 创建统一任务获取器
func NewUnifiedTaskFetcher(
	cfg *config.Config,
	managementClient *management.ClientManager,
	submitters map[string]TaskSubmitter,
	logger *logrus.Logger,
) *TaskFetcher {
	interval := time.Duration(cfg.Worker.TaskInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	fetcher := &TaskFetcher{
		BaseComponent:    lifecycle.NewBaseComponent("TaskFetcher", []string{}, 50),
		config:           cfg,
		managementClient: managementClient,
		submitters:       submitters,
		interval:         interval,
		processingTasks:  make(map[string]time.Time),
		logger:           logger,
	}

	// 创建子服务
	fetcher.cleanupService = NewCleanupService(fetcher, cfg)
	fetcher.monitorService = NewMonitorService(fetcher)

	return fetcher
}

// Start 启动任务获取器
func (f *TaskFetcher) Start(ctx context.Context) error {
	if f.IsRunning() {
		return errors.New(errors.ErrCodeSystem, "TaskFetcher已在运行")
	}

	f.logger.Infof("启动统一任务获取器，间隔: %v", f.interval)

	// 创建子上下文
	f.ctx, f.cancel = context.WithCancel(ctx)

	// 启动子服务
	f.wg.Add(2)
	go f.runCleanupService()
	go f.runMonitorService()

	// 启动主循环
	f.wg.Add(1)
	go f.runMainLoop()

	f.SetRunning(true)
	f.logger.Info("统一任务获取器启动完成")
	return nil
}

// Stop 停止任务获取器
func (f *TaskFetcher) Stop(ctx context.Context) error {
	if !f.IsRunning() {
		f.logger.Info("TaskFetcher未运行，无需停止")
		return nil
	}

	f.logger.Info("开始停止统一任务获取器...")

	// 取消子上下文
	if f.cancel != nil {
		f.cancel()
	}

	// 等待所有goroutine完成
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.wg.Wait()
	}()

	select {
	case <-done:
		f.logger.Info("所有任务获取器goroutine已停止")
	case <-ctx.Done():
		f.logger.Warn("等待任务获取器停止超时")
		return errors.New(errors.ErrCodeTimeout, "停止TaskFetcher超时")
	}

	f.SetRunning(false)
	f.logger.Info("统一任务获取器停止完成")
	return nil
}

// runMainLoop 运行主循环
func (f *TaskFetcher) runMainLoop() {
	defer f.wg.Done()

	// 启动时立即执行一次任务获取
	f.logger.Info("🚀 启动时立即执行首次任务获取")
	f.fetchAndDispatchTasks(f.ctx)

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			f.logger.Info("任务获取主循环停止")
			return
		case <-ticker.C:
			f.fetchAndDispatchTasks(f.ctx)
		}
	}
}

// runCleanupService 运行清理服务
func (f *TaskFetcher) runCleanupService() {
	defer f.wg.Done()
	f.cleanupService.Start(f.ctx)
}

// runMonitorService 运行监控服务
func (f *TaskFetcher) runMonitorService() {
	defer f.wg.Done()
	f.monitorService.StartMonitoring(f.ctx)
}

// RemoveProcessingTask 从处理中任务列表移除（任务完成或失败时调用）
func (f *TaskFetcher) RemoveProcessingTask(taskID string) {
	f.tasksMutex.Lock()
	delete(f.processingTasks, taskID)
	f.tasksMutex.Unlock()
	logrus.Infof("✅ 任务已从处理队列移除: TaskID=%s", taskID)
}

// OnTaskCompleted 实现 TaskCompletionNotifier 接口
func (f *TaskFetcher) OnTaskCompleted(taskID int64) {
	f.RemoveProcessingTask(fmt.Sprintf("%d", taskID))
}

// GetLongRunningTasks 获取超过指定阈值的长时间运行任务
func (f *TaskFetcher) GetLongRunningTasks(threshold time.Duration) []QueueTaskInfo {
	f.tasksMutex.RLock()
	defer f.tasksMutex.RUnlock()

	now := time.Now()
	result := make([]QueueTaskInfo, 0)
	for taskID, submitTime := range f.processingTasks {
		if now.Sub(submitTime) > threshold {
			result = append(result, QueueTaskInfo{ID: taskID, Duration: now.Sub(submitTime)})
		}
	}
	return result
}
