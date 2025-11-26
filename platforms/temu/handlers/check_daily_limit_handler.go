package handlers

import (
	"fmt"
	"time"

	"task-processor/common/memory"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// CheckDailyLimitHandler 检查每日上架限制处理器（参考SHEIN实现）
type CheckDailyLimitHandler struct {
	memoryManager *memory.MemoryManager
	logger        *logrus.Entry
}

// NewCheckDailyLimitHandler 创建新的检查每日限制处理器
func NewCheckDailyLimitHandler(memoryManager *memory.MemoryManager) *CheckDailyLimitHandler {
	return &CheckDailyLimitHandler{
		memoryManager: memoryManager,
		logger:        logrus.WithField("handler", "CheckDailyLimitHandler"),
	}
}

// Name 返回处理器名称
func (h *CheckDailyLimitHandler) Name() string {
	return "检查每日上架限制"
}

// Handle 执行检查每日上架限制处理
func (h *CheckDailyLimitHandler) Handle(ctx *pipeline.TaskContext) error {
	// 检查必要的上下文信息
	if h.memoryManager == nil {
		h.logger.Debug("内存管理器未初始化，跳过每日限制检查")
		return nil
	}

	if ctx.Task == nil {
		h.logger.Debug("任务信息未初始化，跳过每日限制检查")
		return nil
	}

	if ctx.StoreInfo == nil {
		h.logger.Debug("店铺信息未初始化，跳过每日限制检查")
		return nil
	}

	// 检查店铺是否有每日上架限额
	if ctx.StoreInfo.DailyLimit == nil || *ctx.StoreInfo.DailyLimit <= 0 {
		h.logger.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", ctx.StoreInfo.ID)
		return nil
	}

	dailyLimit := *ctx.StoreInfo.DailyLimit
	h.logger.Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", ctx.StoreInfo.ID, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := time.Now().Format("2006-01-02")

	// 获取当前已上架数量
	currentCount := h.memoryManager.DailyCountManager.GetCount(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		currentDate,
	)

	// 计算本次发布会增加的数量
	increment := h.calculateIncrement(ctx)
	if increment <= 0 {
		h.logger.Warnf("计算增量失败，跳过限制检查")
		return nil
	}

	// 预测发布后的总数量
	predictedCount := currentCount + increment

	h.logger.Infof("店铺 %d 在 %s 的上架情况: 当前=%d, 本次增加=%d, 预计=%d, 限额=%d (类型: %s)",
		ctx.StoreInfo.ID, currentDate, currentCount, increment, predictedCount, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 检查是否会超过限额
	if predictedCount > int64(dailyLimit) {
		h.logger.Warnf("店铺 %d 在 %s 的上架数量即将超过限额: 当前=%d, 本次增加=%d, 预计=%d, 限额=%d",
			ctx.StoreInfo.ID, currentDate, currentCount, increment, predictedCount, dailyLimit)

		// 暂停店铺上架到当日结束
		if err := h.pauseShopUntilEndOfDay(
			ctx,
			fmt.Sprintf("达到每日上架限额(%d/%d)", currentCount, dailyLimit),
		); err != nil {
			h.logger.Errorf("暂停店铺上架失败: %v", err)
		}

		// 返回不可重试错误，阻止产品发布
		return types.NewNonRetryableError(
			fmt.Sprintf("店铺已达到每日上架限额(%d/%d)，已暂停上架到当日结束", currentCount, dailyLimit),
			nil,
		)
	}

	h.logger.Infof("店铺 %d 在 %s 的上架数量未超过限额，允许继续发布", ctx.StoreInfo.ID, currentDate)
	return nil
}

// calculateIncrement 根据店铺配置的限制类型计算增量
func (h *CheckDailyLimitHandler) calculateIncrement(ctx *pipeline.TaskContext) int64 {
	switch ctx.StoreInfo.DailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：根据TEMU产品的SKC数量计算
		if ctx.TemuProduct != nil && len(ctx.TemuProduct.SkcList) > 0 {
			skcCount := int64(len(ctx.TemuProduct.SkcList))
			h.logger.Debugf("SKC计数: %d", skcCount)
			return skcCount
		}
		// 如果没有TEMU产品数据，尝试从Amazon变体估算
		if len(ctx.AmazonVariants) > 0 {
			return int64(len(ctx.AmazonVariants))
		}
		// 至少算1个
		return 1
	case "SKU":
		// SKU级别：根据所有SKU数量计算
		if ctx.TemuProduct != nil && len(ctx.TemuProduct.SkcList) > 0 {
			var skuCount int64
			for _, skc := range ctx.TemuProduct.SkcList {
				skuCount += int64(len(skc.SkuList))
			}
			h.logger.Debugf("SKU计数: %d", skuCount)
			return skuCount
		}
		// 如果没有TEMU产品数据，尝试从Amazon变体估算
		if len(ctx.AmazonVariants) > 0 {
			return int64(len(ctx.AmazonVariants))
		}
		// 至少算1个
		return 1
	default:
		// 默认按SPU计算
		h.logger.Warnf("未知的限制类型: %s，默认按SPU计算", ctx.StoreInfo.DailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束
func (h *CheckDailyLimitHandler) pauseShopUntilEndOfDay(ctx *pipeline.TaskContext, reason string) error {
	// 暂停店铺到当日结束（23:59:59）
	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", ctx.Task.TenantID, ctx.Task.StoreID, reason)

	return nil
}
