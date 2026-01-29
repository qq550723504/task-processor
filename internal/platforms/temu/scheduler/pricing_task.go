// Package scheduler 提供TEMU平台核价任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/services/pricing"

	"github.com/sirupsen/logrus"
)

// PricingTask TEMU核价任务
type PricingTask struct {
	*BaseTask
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewPricingTask 创建核价任务
func NewPricingTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
) *PricingTask {
	baseTask := NewBaseTask(config)

	return &PricingTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuPricingTask",
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TenantID,
			"store_id":  config.StoreID,
		}),
	}
}

// Execute 执行核价任务
func (t *PricingTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行TEMU智能核价任务")

	// 创建API客户端
	apiClient := api.NewAPIClient(t.GetStoreID(), t.managementClient)
	if apiClient == nil {
		return fmt.Errorf("创建TEMU API客户端失败")
	}

	// 创建自动核价服务
	autoPricingService := pricing.NewAutoPricingService(apiClient)

	// 执行核价任务
	var stats interface{}
	var err error
	stats, err = autoPricingService.AutoProcessPendingPricesWithRules(t.managementClient)

	if err != nil {
		t.logger.WithError(err).Error("TEMU智能核价任务执行失败")
		return err
	}

	// 记录统计信息
	t.logStatistics(stats)

	return nil
}

// logStatistics 记录统计信息
func (t *PricingTask) logStatistics(stats interface{}) {
	// 尝试类型断言获取统计信息
	type PricingStats interface {
		GetTotalProcessed() int
		GetAcceptCount() int
		GetRejectCount() int
		GetReappealCount() int
		GetSkipCount() int
	}

	if s, ok := stats.(PricingStats); ok {
		t.logger.Infof("🎉 TEMU智能核价任务执行成功，统计: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d",
			s.GetTotalProcessed(), s.GetAcceptCount(), s.GetRejectCount(), s.GetReappealCount(), s.GetSkipCount())
	} else {
		t.logger.Info("TEMU智能核价任务执行成功")
	}
}
