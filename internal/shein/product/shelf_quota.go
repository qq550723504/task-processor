package product

import (
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// ShelfQuotaHandler SKC上架额度检查处理器
type ShelfQuotaHandler struct {
}

// NewShelfQuotaHandler 创建新的SKC上架额度检查处理器
func NewShelfQuotaHandler() *ShelfQuotaHandler {
	return &ShelfQuotaHandler{}
}

// Name 返回处理器名称
func (h *ShelfQuotaHandler) Name() string {
	return "检查SKC上架额度"
}

// submitRemainingShelfQuota 提交剩余SKC上架额度
func (h *ShelfQuotaHandler) submitRemainingShelfQuota(ctx *shein.TaskContext, remainCount int) {
	// 通过DailyCountManager获取客户端
	client := ctx.MemoryManager.DailyCountManager.GetClient()
	if client == nil {
		logrus.Warn("每日上架数量客户端未初始化，无法提交剩余SKC上架额度")
		return
	}

	// 这里可以根据需要扩展客户端接口来支持SKC上架额度
	// 目前先记录日志
	logrus.Infof("SKC上架额度信息已获取，剩余额度: %d", remainCount)
}

// Handle 处理SKC上架额度检查
func (h *ShelfQuotaHandler) Handle(ctx *shein.TaskContext) error {
	// 调用SKC上架额度查询接口
	shelfQuotaResp, err := ctx.OtherAPI.QueryShelfQuota()
	if err != nil {
		logrus.Infof("获取SKC上架额度失败: %v", err)
		return err
	}

	// 输出日志信息
	logrus.Infof("SKC上架额度信息: 需要检查=%t, 剩余数量=%d, 总配额=%d, 已上架数量=%d",
		shelfQuotaResp.Info.Need,
		shelfQuotaResp.Info.RemainCount,
		shelfQuotaResp.Info.TotalQuotaCount,
		shelfQuotaResp.Info.OnShelfCount)

	// 提交剩余SKC上架额度到管理系统
	if ctx.MemoryManager != nil && ctx.MemoryManager.DailyCountManager != nil {
		h.submitRemainingShelfQuota(ctx, shelfQuotaResp.Info.RemainCount)
	}

	// 如果需要检查配额且剩余配额不足
	if shelfQuotaResp.Info.Need && shelfQuotaResp.Info.RemainCount < 1 {
		logrus.Warnf("SKC上架额度不足，暂停店铺并终止任务: 需要检查=%t, 剩余配额=%d",
			shelfQuotaResp.Info.Need, shelfQuotaResp.Info.RemainCount)

		// 设置店铺暂停状态到当日结束
		if ctx.MemoryManager != nil && ctx.MemoryManager.ShopPauseManager != nil {
			ctx.MemoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
				ctx.Task.TenantID,
				ctx.Task.StoreID,
				"SKC上架额度不足",
			)
		}

		return shein.NewFilteredError("SKC上架额度不足")
	}

	// 将SKC上架额度信息保存到上下文中，供后续步骤使用
	ctx.ShelfQuotaInfo = &shelfQuotaResp.Info

	return nil
}
