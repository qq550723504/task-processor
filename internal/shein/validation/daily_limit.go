package validation

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	"task-processor/internal/pkg/timex"
	"task-processor/internal/shein"
)

// CheckDailyLimitHandler 检查每日上架限制处理器（在发布前检查）
type CheckDailyLimitHandler struct {
}

// NewCheckDailyLimitHandler 创建新的检查每日限制处理器
func NewCheckDailyLimitHandler() *CheckDailyLimitHandler {
	return &CheckDailyLimitHandler{}
}

// Name 返回处理器名称
func (h *CheckDailyLimitHandler) Name() string {
	return "检查每日上架限制"
}

// Handle 执行检查每日上架限制处理
func (h *CheckDailyLimitHandler) Handle(ctx *shein.TaskContext) error {
	// 检查必要的上下文信息
	if ctx.MemoryManager == nil {
		logger.GetGlobalLogger("shein/validation").Debug("内存管理器未初始化，跳过每日限制检查")
		return nil
	}

	if ctx.Task == nil {
		logger.GetGlobalLogger("shein/validation").Debug("任务信息未初始化，跳过每日限制检查")
		return nil
	}

	if ctx.StoreInfo == nil {
		logger.GetGlobalLogger("shein/validation").Debug("店铺信息未初始化，跳过每日限制检查")
		return nil
	}

	// 检查店铺是否有每日上架限额
	if ctx.StoreInfo.DailyLimit == nil || *ctx.StoreInfo.DailyLimit <= 0 {
		logger.GetGlobalLogger("shein/validation").Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", ctx.StoreInfo.ID)
		return nil
	}

	dailyLimit := *ctx.StoreInfo.DailyLimit
	logger.GetGlobalLogger("shein/validation").Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", ctx.StoreInfo.ID, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := timex.NowDate()

	// 计算本次发布会增加的数量
	increment := h.calculateIncrement(ctx)
	if increment <= 0 {
		logger.GetGlobalLogger("shein/validation").Warnf("计算增量失败，跳过限制检查")
		return nil
	}

	reservation, err := ctx.MemoryManager.DailyCountManager.TryReserveQuota(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		currentDate,
		increment,
		int64(dailyLimit),
	)
	if err != nil {
		return err
	}

	currentCount := reservation.NewCount
	predictedCount := reservation.NewCount
	if reservation.Allowed {
		currentCount = reservation.NewCount - increment
	}
	if !reservation.Allowed {
		predictedCount = reservation.NewCount + increment
	}

	logger.GetGlobalLogger("shein/validation").Infof("店铺 %d 在 %s 的上架情况: 当前=%d, 本次增加=%d, 预计=%d, 限额=%d (类型: %s)",
		ctx.StoreInfo.ID, currentDate, currentCount, increment, predictedCount, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 检查是否会超过限额
	if !reservation.Allowed {
		logger.GetGlobalLogger("shein/validation").Warnf("店铺 %d 在 %s 的上架数量即将超过限额: 当前=%d, 本次增加=%d, 预计=%d, 限额=%d",
			ctx.StoreInfo.ID, currentDate, currentCount, increment, predictedCount, dailyLimit)

		// 暂停店铺上架并清理相关缓存（暂停到当日结束）
		h.pauseShopUntilEndOfDay(
			ctx,
			fmt.Sprintf("达到每日上架限额(%d/%d)", currentCount, dailyLimit),
		)

		// 店铺级阻塞落 paused，由 pipeline 统一更新主任务状态。
		return shein.NewTaskHandledError(
			model.TaskStatusPaused,
			fmt.Sprintf("店铺已达到每日上架限额(%d/%d)，已暂停上架到当日结束", currentCount, dailyLimit),
			shein.FormatTaskStageMessage(
				ctx.GetStage(),
				shein.FormatTaskReasonMessage(
					shein.TaskReasonDailyLimitReached,
					fmt.Sprintf("店铺已达到每日上架限额(%d/%d)，已暂停上架到当日结束", currentCount, dailyLimit),
				),
			),
		)
	}

	ctx.SetDailyQuotaReservation(currentDate, increment)

	logger.GetGlobalLogger("shein/validation").Infof("店铺 %d 在 %s 的上架数量未超过限额，允许继续发布", ctx.StoreInfo.ID, currentDate)
	return nil
}

// calculateIncrement 根据店铺配置的限制类型计算增量
func (h *CheckDailyLimitHandler) calculateIncrement(ctx *shein.TaskContext) int64 {
	return EstimateListingIncrement(ctx)
}

// EstimateListingIncrement 估算本次上架会增加的计数。
// 发布前调用时依赖 AmazonProduct/Variants 估算；发布后若 SheinResponse 已填充则使用精确值。
func EstimateListingIncrement(ctx *shein.TaskContext) int64 {
	if ctx.StoreInfo == nil {
		return 1
	}
	log := logger.GetGlobalLogger("shein/validation")

	switch ctx.StoreInfo.DailyLimitType {
	case "SPU":
		return 1

	case "SKC":
		// 发布后：优先使用 SheinResponse 精确值
		if ctx.SheinResponse != nil && len(ctx.SheinResponse.Info.SKCList) > 0 {
			return int64(len(ctx.SheinResponse.Info.SKCList))
		}
		// 发布前：从 VariationsValues 中找颜色维度估算
		if ctx.AmazonProduct != nil {
			for _, variation := range ctx.AmazonProduct.VariationsValues {
				name := variation.VariantName
				if name == "Color" || name == "Colour" || name == "Style" || name == "Color Name" {
					count := int64(len(variation.Values))
					log.Debugf("SKC计数: 找到颜色规格 %s，数量=%d", name, count)
					return count
				}
			}
		}
		if ctx.Variants != nil && len(*ctx.Variants) > 0 {
			return int64(len(*ctx.Variants))
		}
		return 1

	case "SKU":
		// 发布后：优先使用 SheinResponse 精确值
		if ctx.SheinResponse != nil {
			var total int64
			for _, skc := range ctx.SheinResponse.Info.SKCList {
				total += int64(len(skc.SKUList))
			}
			if total > 0 {
				return total
			}
		}
		// 发布前：从 Variations 或 Variants 估算
		if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
			count := int64(len(ctx.AmazonProduct.Variations))
			log.Debugf("SKU计数: 根据Variations数组计算，数量=%d", count)
			return count
		}
		if ctx.Variants != nil && len(*ctx.Variants) > 0 {
			count := int64(len(*ctx.Variants))
			log.Debugf("SKU计数: 根据Variants列表计算，数量=%d", count)
			return count
		}
		return 1

	default:
		log.Warnf("未知的限制类型: %s，默认按SPU计算", ctx.StoreInfo.DailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束并清理相关缓存
func (h *CheckDailyLimitHandler) pauseShopUntilEndOfDay(ctx *shein.TaskContext, reason string) {
	if ctx.MemoryManager != nil {
		logger.GetGlobalLogger("shein/validation").Infof("正在清理店铺 %d:%d 的相关缓存", ctx.Task.TenantID, ctx.Task.StoreID)
	}

	ctx.MemoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		reason,
	)

	logger.GetGlobalLogger("shein/validation").Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", ctx.Task.TenantID, ctx.Task.StoreID, reason)
}
