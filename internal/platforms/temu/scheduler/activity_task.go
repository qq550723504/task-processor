// Package scheduler 提供TEMU平台活动报名任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

// ActivityTask TEMU活动报名任务
type ActivityTask struct {
	*BaseTask
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewActivityTask 创建活动报名任务
func NewActivityTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
) *ActivityTask {
	baseTask := NewBaseTask(config)

	return &ActivityTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuActivityTask",
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TenantID,
			"store_id":  config.StoreID,
		}),
	}
}

// Execute 执行活动报名任务
func (t *ActivityTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行TEMU活动报名任务")

	// TODO: 实现TEMU活动报名逻辑

	t.logger.Info("TEMU活动报名任务执行完成")
	return nil
}
