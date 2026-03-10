package product

import (
	"fmt"

	"task-processor/internal/application/state"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/timeutil"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// CheckDailyLimitHandler 检查每日上架限制处理器（参考SHEIN实现）
type CheckDailyLimitHandler struct {
	memoryManager *state.MemoryManager
	logger        *logrus.Entry
}

// NewCheckDailyLimitHandler 创建新的检查每日限制处理器
func NewCheckDailyLimitHandler(memoryManager *state.MemoryManager) *CheckDailyLimitHandler {
	return &CheckDailyLimitHandler{
		memoryManager: memoryManager,
		logger:        logger.GetGlobalLogger("temu.handlers.daily_limit").WithField("handler", "CheckDailyLimitHandler"),
	}
}

// Name 返回处理器名称
func (h *CheckDailyLimitHandler) Name() string {
	return "检查每日上架限制"
}

// Handle 执行检查每日上架限制处理（兼容pipeline.Handler接口）
func (h *CheckDailyLimitHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 执行检查每日上架限制处理（强类型上下文）
func (h *CheckDailyLimitHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 检查必要的上下文信息
	if h.memoryManager == nil {
		h.logger.Debug("内存管理器未初始化，跳过每日限制检查")
		return nil
	}

	task := temuCtx.GetTask()
	if task == nil {
		h.logger.Debug("任务信息未初始化，跳过每日限制检查")
		return nil
	}

	// 从强类型上下文获取店铺信息
	if temuCtx.StoreInfo == nil {
		h.logger.Debug("店铺信息未初始化，跳过每日限制检查")
		return nil
	}

	storeInfo := temuCtx.StoreInfo

	// 检查店铺是否有每日上架限额
	if storeInfo.DailyLimit == nil || *storeInfo.DailyLimit <= 0 {
		h.logger.WithField(logger.FieldStoreID, task.StoreID).Debug("店铺没有设置每日上架限额，跳过限额检查")
		return nil
	}

	dailyLimit := *storeInfo.DailyLimit
	dailyLimitType := storeInfo.DailyLimitType
	if dailyLimitType == "" {
		dailyLimitType = "SPU"
	}

	h.logger.WithFields(map[string]interface{}{
		logger.FieldStoreID: task.StoreID,
		"daily_limit":       dailyLimit,
		"limit_type":        dailyLimitType,
	}).Debug("店铺每日上架限额")

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := timeutil.NowDate()

	// 获取当前已上架数量
	currentCount := h.memoryManager.DailyCountManager.GetCount(
		task.TenantID,
		task.StoreID,
		currentDate,
	)

	// 计算本次发布会增加的数量
	increment := h.calculateIncrement(temuCtx, dailyLimitType)
	if increment <= 0 {
		h.logger.Warn("计算增量失败，跳过限制检查")
		return nil
	}

	// 预测发布后的总数量
	predictedCount := currentCount + increment

	h.logger.WithFields(map[string]interface{}{
		logger.FieldStoreID: task.StoreID,
		"date":              currentDate,
		"current_count":     currentCount,
		"increment":         increment,
		"predicted_count":   predictedCount,
		"daily_limit":       dailyLimit,
		"limit_type":        dailyLimitType,
	}).Info("店铺上架情况")

	// 检查是否会超过限额
	if predictedCount > int64(dailyLimit) {
		h.logger.WithFields(map[string]interface{}{
			logger.FieldStoreID: task.StoreID,
			"date":              currentDate,
			"current_count":     currentCount,
			"increment":         increment,
			"predicted_count":   predictedCount,
			"daily_limit":       dailyLimit,
		}).Warn("店铺上架数量即将超过限额")

		// 暂停店铺上架到当日结束
		if err := h.pauseShopUntilEndOfDay(
			temuCtx,
			fmt.Sprintf("达到每日上架限额(%d/%d)", currentCount, dailyLimit),
		); err != nil {
			h.logger.WithError(err).Error("暂停店铺上架失败")
		}

		// 返回不可重试错误，阻止产品发布
		return types.NewNonRetryableError(
			fmt.Sprintf("店铺已达到每日上架限额(%d/%d)，已暂停上架到当日结束", currentCount, dailyLimit),
			nil,
		)
	}

	h.logger.WithFields(map[string]interface{}{
		logger.FieldStoreID: task.StoreID,
		"date":              currentDate,
	}).Info("店铺上架数量未超过限额，允许继续发布")
	return nil
}

// calculateIncrement 根据店铺配置的限制类型计算增量
func (h *CheckDailyLimitHandler) calculateIncrement(temuCtx *temucontext.TemuTaskContext, dailyLimitType string) int64 {
	switch dailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：根据TEMU产品的SKC数量计算
		if temuCtx.TemuProduct != nil {
			skcCount := int64(len(temuCtx.TemuProduct.SkcList))
			h.logger.Debugf("SKC计数: %d", skcCount)
			return skcCount
		}
		// 如果没有TEMU产品数据，尝试从Amazon变体估算
		variants := temuCtx.GetVariants()
		if len(variants) > 0 {
			return int64(len(variants))
		}
		// 至少算1个
		return 1
	case "SKU":
		// SKU级别：根据所有SKU数量计算
		if temuCtx.TemuProduct != nil {
			var skuCount int64
			for _, skc := range temuCtx.TemuProduct.SkcList {
				skuCount += int64(len(skc.SkuList))
			}
			h.logger.Debugf("SKU计数: %d", skuCount)
			return skuCount
		}
		// 如果没有TEMU产品数据，尝试从Amazon变体估算
		variants := temuCtx.GetVariants()
		if len(variants) > 0 {
			return int64(len(variants))
		}
		// 至少算1个
		return 1
	default:
		// 默认按SPU计算
		h.logger.Warnf("未知的限制类型: %s，默认按SPU计算", dailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束
func (h *CheckDailyLimitHandler) pauseShopUntilEndOfDay(temuCtx *temucontext.TemuTaskContext, reason string) error {
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 暂停店铺到当日结束（23:59:59）
	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		task.TenantID,
		task.StoreID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", task.TenantID, task.StoreID, reason)

	return nil
}
