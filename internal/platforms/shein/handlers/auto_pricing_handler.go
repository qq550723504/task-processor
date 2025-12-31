// Package handlers 提供SHEIN平台的自动核价处理功能
package handlers

import (
	"context"
	"time"

	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/scheduler"

	"github.com/sirupsen/logrus"
)

// AutoPricingHandler 自动核价处理器，采用调度器模式
type AutoPricingHandler struct {
	schedulerManager *scheduler.SchedulerManager
	managementClient *management.ClientManager
	storeIDs         []int64
	logger           *logrus.Entry
}

// NewAutoPricingHandler 创建新的自动核价处理器
func NewAutoPricingHandler(managementClient *management.ClientManager, storeIDs []int64) *AutoPricingHandler {
	return &AutoPricingHandler{
		managementClient: managementClient,
		storeIDs:         storeIDs,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SHEINAutoPricingHandler",
		}),
	}
}

// Start 启动自动核价任务
func (h *AutoPricingHandler) Start(ctx context.Context, interval time.Duration) {
	h.logger.Infof("启动SHEIN自动核价处理器，间隔: %v, 店铺数量: %d", interval, len(h.storeIDs))

	// 创建调度器管理器
	h.schedulerManager = scheduler.NewSchedulerManager(ctx, h.managementClient, interval)

	// 第一步：预热所有店铺的Cookie
	h.logger.Info("开始预热店铺Cookie...")
	prewarmResults := h.schedulerManager.PrewarmCookies(h.storeIDs)

	// 第二步：为预热成功的店铺添加调度器
	successCount := 0
	skipCount := 0
	errorCount := 0

	for _, result := range prewarmResults {
		if !result.Success {
			h.logger.Warnf("店铺 %s (ID: %d) Cookie预热失败，跳过添加调度器: %s",
				result.StoreName, result.StoreID, result.ErrorMessage)
			skipCount++
			continue
		}

		// 为预热成功的店铺添加调度器
		if err := h.schedulerManager.AddStore(result.TenantID, result.StoreID); err != nil {
			h.logger.Errorf("添加店铺 %d 调度器失败: %v", result.StoreID, err)
			errorCount++
			continue
		}

		h.logger.Infof("✅ 成功为店铺 %s (ID: %d) 添加自动核价调度器", result.StoreName, result.StoreID)
		successCount++
	}

	h.logger.Infof("SHEIN自动核价处理器启动完成 - 成功: %d, 跳过: %d, 失败: %d, 总调度器数: %d",
		successCount, skipCount, errorCount, h.schedulerManager.GetSchedulerCount())
}

// Stop 停止自动核价任务
func (h *AutoPricingHandler) Stop() {
	h.logger.Info("停止SHEIN自动核价处理器")

	if h.schedulerManager != nil {
		h.schedulerManager.StopAll()
	}

	h.logger.Info("SHEIN自动核价处理器已停止")
}
