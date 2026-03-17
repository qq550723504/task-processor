// package scheduler 提供SHEIN平台活动报名任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/client"
	schedulerservice "task-processor/internal/shein/operation"

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

	// 1. 获取运营策略，判断活动类型
	strategyClient := t.managementClient.GetOperationStrategyClient()
	strategy, err := strategyClient.GetOperationStrategyByStoreId(t.GetStoreID())
	if err != nil {
		t.logger.WithError(err).Warn("获取运营策略失败，跳过任务")
		return nil
	}

	// 检查策略是否启用活动功能
	if strategy == nil || !strategy.IsEnabled() || !strategy.ActivityEnabled {
		t.logger.Info("运营策略未启用活动功能，跳过任务")
		return nil
	}

	// 2. 根据活动类型执行不同的逻辑
	switch strategy.ActivityType {
	case "PROMOTION":
		return t.executePromotionActivity(ctx, strategy)
	case "TIME_LIMITED":
		return t.executeTimeLimitedDiscountActivity(ctx, strategy)
	case "MIXED":
		return t.executeMixedActivity(ctx, strategy)
	default:
		t.logger.Warnf("未知的活动类型: %s，跳过任务", strategy.ActivityType)
		return nil
	}
}

// executePromotionActivity 执行促销活动报名
func (t *ActivityTask) executePromotionActivity(ctx context.Context, strategy *managementapi.OperationStrategyDTO) error {
	t.logger.Info("执行促销活动报名")

	// 根据运营策略报名促销活动（内部会获取产品列表、构建配置、提交报名）
	registeredCount, err := t.activityService.RegisterPromotionActivity(ctx, strategy)
	if err != nil {
		return fmt.Errorf("报名促销活动失败: %w", err)
	}

	// 记录统计信息
	t.logger.WithFields(logrus.Fields{
		"registered_products": registeredCount,
	}).Info("促销活动报名任务执行完成")

	return nil
}

// executeTimeLimitedDiscountActivity 执行限时折扣活动创建
func (t *ActivityTask) executeTimeLimitedDiscountActivity(ctx context.Context, strategy *managementapi.OperationStrategyDTO) error {
	t.logger.Info("执行限时折扣活动创建")

	// 根据运营策略创建限时折扣活动（内部会查询商品、计算价格、创建活动）
	registeredCount, err := t.activityService.CreateTimeLimitedDiscountActivity(ctx, strategy)
	if err != nil {
		return fmt.Errorf("创建限时折扣活动失败: %w", err)
	}

	// 记录统计信息
	t.logger.WithFields(logrus.Fields{
		"registered_products": registeredCount,
	}).Info("限时折扣活动创建完成")

	return nil
}

// executeMixedActivity 执行混合活动（按比例分配促销和限时折扣）
func (t *ActivityTask) executeMixedActivity(ctx context.Context, strategy *managementapi.OperationStrategyDTO) error {
	t.logger.WithField("promotion_ratio", strategy.PromotionRatio).Info("执行混合活动")

	// 根据运营策略按比例执行混合活动
	promotionCount, timeLimitedCount, err := t.activityService.RegisterMixedActivity(ctx, strategy)
	if err != nil {
		return fmt.Errorf("执行混合活动失败: %w", err)
	}

	// 记录统计信息
	t.logger.WithFields(logrus.Fields{
		"promotion_count":    promotionCount,
		"time_limited_count": timeLimitedCount,
		"total_count":        promotionCount + timeLimitedCount,
	}).Info("混合活动执行完成")

	return nil
}
