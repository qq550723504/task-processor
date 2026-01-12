// Package scheduler 提供TEMU平台的调度器实现
package scheduler

import (
	"context"
	"fmt"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/services/pricing"
	"time"

	"github.com/sirupsen/logrus"
)

// createTemuPricingTask 创建TEMU核价任务
func createTemuPricingTask(
	clientManager *management.ClientManager,
	storeID int64,
	configProvider temu.ConfigProvider,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger := logrus.WithFields(logrus.Fields{
			"component": "TemuPricingTask",
			"storeID":   storeID,
		})

		logger.Info("开始执行TEMU智能核价任务")

		// 创建API客户端
		apiClient := api.NewAPIClient(storeID, clientManager)
		if apiClient == nil {
			return fmt.Errorf("创建TEMU API客户端失败")
		}

		// 创建自动核价服务
		autoPricingService := pricing.NewAutoPricingService(apiClient)

		// 执行核价任务
		var stats *models.PricingStatistics
		var err error

		if configProvider != nil {
			// 使用Amazon增强版本
			logger.Info("使用Amazon增强版核价方法")
			stats, err = autoPricingService.AutoProcessPendingPricesWithRulesAndAmazon(clientManager, configProvider)
		} else {
			// 使用基础版本
			logger.Info("使用基础版核价方法")
			stats, err = autoPricingService.AutoProcessPendingPricesWithRules(clientManager)
		}

		if err != nil {
			logger.WithError(err).Error("TEMU智能核价任务执行失败")
			return err
		}

		logger.Infof("🎉 TEMU智能核价任务执行成功，统计: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d",
			stats.TotalProcessed, stats.AcceptCount, stats.RejectCount, stats.ReappealCount, stats.SkipCount)

		return nil
	}
}

// AddTemuPricingTask 添加TEMU核价任务到调度器
func AddTemuPricingTask(
	scheduler *SafeScheduler,
	clientManager *management.ClientManager,
	storeID int64,
	interval time.Duration,
	configProvider temu.ConfigProvider,
) {
	taskID := fmt.Sprintf("temu_pricing_%d", storeID)
	taskName := fmt.Sprintf("TEMU核价任务 (店铺:%d)", storeID)

	task := &ScheduledTask{
		ID:       taskID,
		Name:     taskName,
		Interval: interval,
		Fn:       createTemuPricingTask(clientManager, storeID, configProvider),
		Enabled:  true,
	}

	scheduler.AddTask(task)
}

// AddTemuPricingTaskBasic 添加基础版TEMU核价任务（不使用Amazon增强）
func AddTemuPricingTaskBasic(
	scheduler *SafeScheduler,
	clientManager *management.ClientManager,
	storeID int64,
	interval time.Duration,
) {
	AddTemuPricingTask(scheduler, clientManager, storeID, interval, nil)
}
