package handlers

import (
	"context"
	"time"

	"task-processor/common/management"
	"task-processor/common/temu"

	"github.com/sirupsen/logrus"
)

// AutoPricingHandler TEMU自动核价处理器
type AutoPricingHandler struct {
	schedulerManager *temu.SchedulerManager
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
			"component": "TEMUAutoPricingHandler",
		}),
	}
}

// Start 启动自动核价任务
func (h *AutoPricingHandler) Start(ctx context.Context, interval time.Duration) {
	h.logger.Infof("启动TEMU自动核价处理器，间隔: %v, 店铺数量: %d", interval, len(h.storeIDs))

	// 使用智能核价模式（根据利润率规则自动决策）
	action := temu.ActionSmart

	// 创建调度器管理器
	h.schedulerManager = temu.NewSchedulerManager(h.managementClient, interval, action)

	// 为每个店铺添加调度器
	successCount := 0
	skipCount := 0
	errorCount := 0

	for _, storeID := range h.storeIDs {
		h.logger.Infof("正在处理店铺 ID: %d", storeID)

		// 获取店铺信息以确定租户ID
		storeInfo, err := h.managementClient.GetStoreClient().GetStore(storeID)
		if err != nil {
			h.logger.Errorf("获取店铺 %d 信息失败: %v", storeID, err)
			errorCount++
			continue
		}

		h.logger.Infof("店铺信息: Name=%s, Platform=%s, TenantID=%d, EnableAutoPrice=%v",
			storeInfo.Name, storeInfo.Platform, storeInfo.TenantID, storeInfo.EnableAutoPrice)

		// 检查平台类型
		if storeInfo.Platform != "TEMU" && storeInfo.Platform != "temu" {
			h.logger.Infof("店铺 %s (ID: %d) 不是TEMU平台 (Platform=%s)，跳过", storeInfo.Name, storeID, storeInfo.Platform)
			skipCount++
			continue
		}

		// 检查是否启用自动核价
		// 注意：后端 API 中 0=启用, 1=禁用，但 JSON 反序列化后 0->false, 1->true
		// 所以这里的逻辑是：false=启用, true=禁用
		if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice {
			h.logger.Infof("店铺 %s (ID: %d) 未启用自动核价 ，跳过", storeInfo.Name, storeID)
			skipCount++
			continue
		}

		// 如果 EnableAutoPrice 为 nil 或 false，表示启用
		if storeInfo.EnableAutoPrice == nil {
			h.logger.Infof("店铺 %s (ID: %d) EnableAutoPrice 未设置，默认启用自动核价", storeInfo.Name, storeID)
		} else {
			h.logger.Infof("店铺 %s (ID: %d) EnableAutoPrice=false，启用自动核价", storeInfo.Name, storeID)
		}

		// 添加店铺调度器
		if err := h.schedulerManager.AddStore(storeInfo.TenantID, storeID); err != nil {
			h.logger.Errorf("添加店铺 %d 调度器失败: %v", storeID, err)
			errorCount++
			continue
		}

		h.logger.Infof("✅ 成功为店铺 %s (ID: %d) 添加自动核价调度器", storeInfo.Name, storeID)
		successCount++
	}

	h.logger.Infof("TEMU自动核价处理器启动完成 - 成功: %d, 跳过: %d, 失败: %d, 总调度器数: %d",
		successCount, skipCount, errorCount, h.schedulerManager.GetSchedulerCount())
}

// Stop 停止自动核价任务
func (h *AutoPricingHandler) Stop() {
	h.logger.Info("停止TEMU自动核价处理器")

	if h.schedulerManager != nil {
		h.schedulerManager.StopAll()
	}

	h.logger.Info("TEMU自动核价处理器已停止")
}
