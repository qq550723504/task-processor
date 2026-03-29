// Package scheduler 提供TEMU平台活动报名任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ActivityService 定义 TEMU 活动任务的执行边界。
type ActivityService interface {
	Execute(ctx context.Context) error
}

type noopActivityService struct {
	logger *logrus.Entry
}

func newNoopActivityService() ActivityService {
	return &noopActivityService{
		logger: logger.GetGlobalLogger("temu/scheduler").WithField("component", "TemuActivityService"),
	}
}

func (s *noopActivityService) Execute(ctx context.Context) error {
	_ = ctx
	s.logger.Info("TEMU activity service 未实现，跳过执行")
	return nil
}

// ActivityTask TEMU活动报名任务
type ActivityTask struct {
	*BaseTask
	activityService ActivityService
	logger          *logrus.Entry
}

// NewActivityTask 创建活动报名任务
func NewActivityTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	activityService ActivityService,
) *ActivityTask {
	_ = ctx
	baseTask := NewBaseTask(config)

	return &ActivityTask{
		BaseTask:        baseTask,
		activityService: activityService,
		logger: logger.GetGlobalLogger("temu/scheduler").WithFields(logrus.Fields{
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

	if t.activityService == nil {
		t.logger.Warn("TEMU activity service 未配置，跳过执行")
		return nil
	}

	if err := t.activityService.Execute(ctx); err != nil {
		return err
	}

	t.logger.Info("TEMU活动报名任务执行完成")
	return nil
}
