// Package scheduler 提供TEMU平台同步任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// SyncTask TEMU产品同步任务
type SyncTask struct {
	*BaseTask
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewSyncTask 创建同步任务
func NewSyncTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
) *SyncTask {
	baseTask := NewBaseTask(config)

	return &SyncTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuSyncTask",
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TenantID,
			"store_id":  config.StoreID,
		}),
	}
}

// Execute 执行同步任务
func (t *SyncTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行TEMU产品同步任务")

	// TODO: 实现TEMU产品同步逻辑

	t.logger.Info("TEMU产品同步任务执行完成")
	return nil
}
