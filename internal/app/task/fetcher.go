// Package task provides the task fetcher lifecycle.
package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

type TaskFetcher struct {
	*lifecycle.BaseComponent
	config               *config.Config
	managementClient     *management.ClientManager
	pendingTaskSource    pendingRuntimeTaskSource
	submitters           map[string]TaskSubmitter
	interval             time.Duration
	processingTasks      map[string]time.Time
	tasksMutex           sync.RWMutex
	statusServiceFactory func(component string) *taskstatus.Service
	claimJournal         *ClaimJournal

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *logrus.Logger

	cleanupService *CleanupService
	monitorService *MonitorService
}

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
		BaseComponent:     lifecycle.NewBaseComponent("TaskFetcher", []string{}, 50),
		config:            cfg,
		managementClient:  managementClient,
		pendingTaskSource: managementClient,
		submitters:        submitters,
		interval:          interval,
		processingTasks:   make(map[string]time.Time),
		logger:            logger,
		claimJournal:      NewClaimJournal(""),
	}

	fetcher.cleanupService = NewCleanupService(fetcher, cfg)
	fetcher.monitorService = NewMonitorService(fetcher)
	return fetcher
}

func (f *TaskFetcher) Start(ctx context.Context) error {
	if f.IsRunning() {
		return errors.New(errors.ErrCodeSystem, "TaskFetcher已在运行")
	}

	f.logger.Infof("启动统一任务获取器，间隔: %v", f.interval)
	f.ctx, f.cancel = context.WithCancel(ctx)

	if err := f.recoverInterruptedClaims(); err != nil {
		f.logger.WithError(err).Warn("恢复中断 claim 任务失败")
	}

	f.wg.Add(2)
	go f.runCleanupService()
	go f.runMonitorService()

	f.wg.Add(1)
	go f.runMainLoop()

	f.SetRunning(true)
	f.logger.Info("统一任务获取器启动完成")
	return nil
}

func (f *TaskFetcher) Stop(ctx context.Context) error {
	if !f.IsRunning() {
		f.logger.Info("TaskFetcher未运行，无需停止")
		return nil
	}

	f.logger.Info("开始停止统一任务获取器...")
	if f.cancel != nil {
		f.cancel()
	}

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

func (f *TaskFetcher) runMainLoop() {
	defer f.wg.Done()

	f.logger.Info("启动时立即执行首次任务获取")
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

func (f *TaskFetcher) runCleanupService() {
	defer f.wg.Done()
	f.cleanupService.Start(f.ctx)
}

func (f *TaskFetcher) runMonitorService() {
	defer f.wg.Done()
	f.monitorService.StartMonitoring(f.ctx)
}

func (f *TaskFetcher) RemoveProcessingTask(taskID string) {
	f.tasksMutex.Lock()
	delete(f.processingTasks, taskID)
	f.tasksMutex.Unlock()

	var numericTaskID int64
	if _, err := fmt.Sscanf(taskID, "%d", &numericTaskID); err == nil {
		f.removeClaimJournalEntry(numericTaskID)
	}

	logger.GetGlobalLogger("app/task").Infof("任务已从处理队列移除: TaskID=%s", taskID)
}

func (f *TaskFetcher) OnTaskCompleted(taskID int64) {
	f.RemoveProcessingTask(fmt.Sprintf("%d", taskID))
}

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
