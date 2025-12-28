// Package task 提供任务获取核心功能
package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/common/management"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// UnifiedTaskFetcher 统一任务获取器
type TaskFetcher struct {
	config           *config.Config
	managementClient *management.ClientManager
	submitters       map[string]TaskSubmitter // platform -> submitter
	interval         time.Duration
	processingTasks  map[string]time.Time // taskID -> 提交时间，用于去重
	tasksMutex       sync.RWMutex         // 保护 processingTasks
}

// NewUnifiedTaskFetcher 创建统一任务获取器
func NewUnifiedTaskFetcher(
	cfg *config.Config,
	managementClient *management.ClientManager,
	submitters map[string]TaskSubmitter,
) *TaskFetcher {
	interval := time.Duration(cfg.Worker.TaskInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	fetcher := &TaskFetcher{
		config:           cfg,
		managementClient: managementClient,
		submitters:       submitters,
		interval:         interval,
		processingTasks:  make(map[string]time.Time),
	}

	return fetcher
}

// Start 启动任务获取
func (f *TaskFetcher) Start(ctx context.Context) {
	logrus.Infof("统一任务获取器启动，间隔: %v", f.interval)

	// 创建统一服务
	cleanupService := NewCleanupService(f, f.config)
	monitorService := NewMonitorService(f)

	// 启动统一服务
	go cleanupService.Start(ctx)
	go monitorService.StartMonitoring(ctx)

	// 启动时立即执行一次任务获取
	logrus.Info("🚀 启动时立即执行首次任务获取")
	f.fetchAndDispatchTasks(ctx)

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("统一任务获取器停止")
			return
		case <-ticker.C:
			f.fetchAndDispatchTasks(ctx)
		}
	}
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
