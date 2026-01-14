package validation

import (
	"fmt"
	"task-processor/internal/platforms/shein/model"
	"time"

	"github.com/sirupsen/logrus"
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
func (h *CheckDailyLimitHandler) Handle(ctx *model.TaskContext) error {
	// 检查必要的上下文信息
	if ctx.MemoryManager == nil {
		logrus.Debug("内存管理器未初始化，跳过每日限制检查")
		return nil
	}

	if ctx.Task == nil {
		logrus.Debug("任务信息未初始化，跳过每日限制检查")
		return nil
	}

	if ctx.StoreInfo == nil {
		logrus.Debug("店铺信息未初始化，跳过每日限制检查")
		return nil
	}

	// 检查店铺是否有每日上架限额
	if ctx.StoreInfo.DailyLimit == nil || *ctx.StoreInfo.DailyLimit <= 0 {
		logrus.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", ctx.StoreInfo.ID)
		return nil
	}

	dailyLimit := *ctx.StoreInfo.DailyLimit
	logrus.Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", ctx.StoreInfo.ID, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := time.Now().Format("2006-01-02")

	// 获取当前已上架数量
	currentCount := ctx.MemoryManager.DailyCountManager.GetCount(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		currentDate,
	)

	// 计算本次发布会增加的数量
	increment := h.calculateIncrement(ctx)
	if increment <= 0 {
		logrus.Warnf("计算增量失败，跳过限制检查")
		return nil
	}

	// 预测发布后的总数量
	predictedCount := currentCount + increment

	logrus.Infof("店铺 %d 在 %s 的上架情况: 当前=%d, 本次增加=%d, 预计=%d, 限额=%d (类型: %s)",
		ctx.StoreInfo.ID, currentDate, currentCount, increment, predictedCount, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 检查是否会超过限额
	if predictedCount > int64(dailyLimit) {
		logrus.Warnf("店铺 %d 在 %s 的上架数量即将超过限额: 当前=%d, 本次增加=%d, 预计=%d, 限额=%d",
			ctx.StoreInfo.ID, currentDate, currentCount, increment, predictedCount, dailyLimit)

		// 暂停店铺上架并清理相关缓存（暂停到当日结束）
		if err := h.pauseShopUntilEndOfDay(
			ctx,
			fmt.Sprintf("达到每日上架限额(%d/%d)", currentCount, dailyLimit),
		); err != nil {
			logrus.Errorf("暂停店铺上架并清理缓存失败: %v", err)
		}

		// 返回不可重试错误，阻止产品发布
		return model.NewNonRetryableError(
			fmt.Sprintf("店铺已达到每日上架限额(%d/%d)，已暂停上架到当日结束", currentCount, dailyLimit),
			nil,
		)
	}

	logrus.Infof("店铺 %d 在 %s 的上架数量未超过限额，允许继续发布", ctx.StoreInfo.ID, currentDate)
	return nil
}

// calculateIncrement 根据店铺配置的限制类型计算增量
func (h *CheckDailyLimitHandler) calculateIncrement(ctx *model.TaskContext) int64 {
	// 在发布前，我们还没有 SheinResponse，所以需要根据原始数据估算

	switch ctx.StoreInfo.DailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：根据主产品的颜色规格数量计算
		if ctx.AmazonProduct != nil && ctx.AmazonProduct.VariationsValues != nil {
			// 计算颜色规格的数量（通常是第一个变体维度）
			for _, variation := range ctx.AmazonProduct.VariationsValues {
				// 查找颜色相关的变体（Color, Colour, Style等）
				variantName := variation.VariantName
				if variantName == "Color" || variantName == "Colour" ||
					variantName == "Style" || variantName == "Color Name" {
					skcCount := int64(len(variation.Values))
					logrus.Debugf("SKC计数: 找到颜色规格 %s，数量=%d", variantName, skcCount)
					return skcCount
				}
			}
		}
		// 如果没有变体信息，检查是否有变体列表
		if ctx.Variants != nil && len(*ctx.Variants) > 0 {
			return int64(len(*ctx.Variants))
		}
		// 如果都没有，至少算1个
		return 1
	case "SKU":
		// SKU级别：需要根据所有SKU数量估算
		// SKU级别：根据 Variations 数组的长度计算
		// Variations 包含了所有实际的变体（每个变体对应一个SKU）
		if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
			skuCount := int64(len(ctx.AmazonProduct.Variations))
			logrus.Debugf("SKU计数: 根据Variations数组计算，数量=%d", skuCount)
			return skuCount
		}
		// 如果没有 Variations，使用 Variants 列表
		if ctx.Variants != nil && len(*ctx.Variants) > 0 {
			skuCount := int64(len(*ctx.Variants))
			logrus.Debugf("SKU计数: 根据Variants列表计算，数量=%d", skuCount)
			return skuCount
		}
		// 如果都没有，至少算1个
		return 1
	default:
		// 默认按SPU计算
		logrus.Warnf("未知的限制类型: %s，默认按SPU计算", ctx.StoreInfo.DailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束并清理相关缓存
func (h *CheckDailyLimitHandler) pauseShopUntilEndOfDay(ctx *model.TaskContext, reason string) error {
	// 1. 清理客户端缓存（通过内存管理器）
	if ctx.MemoryManager != nil {
		// 通过内存管理器清理相关缓存
		logrus.Infof("正在清理店铺 %d:%d 的相关缓存", ctx.Task.TenantID, ctx.Task.StoreID)
	}

	// 2. 暂停店铺到当日结束（23:59:59）
	ctx.MemoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		reason,
	)

	logrus.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", ctx.Task.TenantID, ctx.Task.StoreID, reason)

	return nil
}
