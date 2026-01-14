// Package scheduler 提供SHEIN平台活动报名任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/repo/client"
	schedulerservice "task-processor/internal/platforms/shein/service/scheduler"

	"github.com/sirupsen/logrus"
)

// ActivityTask SHEIN活动报名任务
type ActivityTask struct {
	*BaseTask
	managementClient *management.ClientManager
	clientManager    *client.ClientManager
	activityService  schedulerservice.ActivityRegistrationService
	logger           *logrus.Entry
}

// NewActivityTask 创建活动报名任务
func NewActivityTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	clientManager *client.ClientManager,
	activityService schedulerservice.ActivityRegistrationService,
) *ActivityTask {
	baseTask := NewBaseTask(config)

	return &ActivityTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		clientManager:    clientManager,
		activityService:  activityService,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SheinActivityTask",
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

	t.logger.Info("开始执行SHEIN活动报名任务")

	// 1. 获取可报名活动的产品列表
	products, err := t.activityService.FetchAvailableProducts(ctx)
	if err != nil {
		return fmt.Errorf("获取可报名产品列表失败: %w", err)
	}

	t.logger.Infof("获取到 %d 个可报名产品", len(products))

	if len(products) == 0 {
		t.logger.Info("没有可报名的产品")
		return nil
	}

	// 2. 自动报名产品到活动
	registeredCount, err := t.activityService.RegisterProducts(ctx, products)
	if err != nil {
		return fmt.Errorf("报名产品失败: %w", err)
	}

	// 3. 记录统计信息
	t.logger.Infof("SHEIN活动报名任务执行完成，成功报名 %d 个产品", registeredCount)
	return nil
}
