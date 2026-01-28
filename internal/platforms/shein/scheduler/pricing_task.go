// Package scheduler 提供SHEIN平台核价任务实现
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

// PricingTask SHEIN核价任务
type PricingTask struct {
	*BaseTask
	managementClient *management.ClientManager
	clientManager    *client.ClientManager
	pricingService   schedulerservice.AutoPricingService
	logger           *logrus.Entry
}

// NewPricingTask 创建核价任务
func NewPricingTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	clientManager *client.ClientManager,
	pricingService schedulerservice.AutoPricingService,
) *PricingTask {
	baseTask := NewBaseTask(config)

	return &PricingTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		clientManager:    clientManager,
		pricingService:   pricingService,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SheinPricingTask",
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

	t.logger.Info("开始执行SHEIN自动核价任务")

	// 获取店铺信息
	storeInfo, err := t.managementClient.GetStoreClient().GetStore(t.GetStoreID())
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 检查是否启用自动核价
	if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice {
		t.logger.Infof("店铺 %s 未启用自动核价功能，跳过", storeInfo.Name)
		return nil
	}

	// 获取店铺API客户端
	_, err = t.clientManager.GetClient(t.GetStoreID(), storeInfo)
	if err != nil {
		return fmt.Errorf("获取店铺API客户端失败: %w", err)
	}

	// 1. 获取待核价产品列表（使用默认时间范围：明天往前推一年）
	pendingProducts, err := t.pricingService.FetchPendingPriceProducts(ctx, "", "")
	if err != nil {
		return fmt.Errorf("获取待核价产品失败: %w", err)
	}

	t.logger.Infof("获取到 %d 个待核价产品", len(pendingProducts))

	// 获取是否启用重新议价配置
	enableRebargain := false
	if storeInfo.EnableRebargain != nil {
		enableRebargain = *storeInfo.EnableRebargain
	}

	// 2. 应用核价规则
	pricingResults, err := t.pricingService.ApplyPricingRules(ctx, pendingProducts, t.GetStoreID(), enableRebargain)
	if err != nil {
		return fmt.Errorf("应用核价规则失败: %w", err)
	}

	// 3. 调用API提交核价结果
	stats, err := t.pricingService.SubmitPricingResults(ctx, pricingResults)
	if err != nil {
		return fmt.Errorf("提交核价结果失败: %w", err)
	}

	// 4. 记录统计信息
	t.logger.Infof("SHEIN核价任务执行完成，统计: 总数=%d, 接受=%d, 拒绝=%d, 重新议价=%d, 跳过=%d",
		stats.TotalProcessed, stats.AcceptCount, stats.RejectCount, stats.ReappealCount, stats.SkipCount)
	return nil
}
