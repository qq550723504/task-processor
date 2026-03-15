package product

import (
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// SpuLimitHandler 检查发品额度处理器
type SpuLimitHandler struct {
}

func NewSpuLimitHandler() *SpuLimitHandler {
	return &SpuLimitHandler{}
}

func (h *SpuLimitHandler) Name() string {
	return "检查发品额度"
}

// submitRemainingQuota 提交剩余发品额度
func (h *SpuLimitHandler) submitRemainingQuota(ctx *model.TaskContext, quota int) {
	// 通过DailyCountManager获取客户端
	client := ctx.MemoryManager.DailyCountManager.GetClient()
	if client == nil {
		logrus.Warn("每日上架数量客户端未初始化，无法提交剩余发品额度")
		return
	}

	success, err := client.SetRemainingListingQuota(ctx.Task.TenantID, ctx.Task.StoreID, quota)
	if err != nil {
		logrus.Warnf("提交剩余发品额度失败: %v", err)
	} else if !success {
		logrus.Warnf("提交剩余发品额度返回失败: storeID=%d, quota=%d", ctx.Task.StoreID, quota)
		// 不影响主流程，继续执行
	} else {
		logrus.Infof("成功提交剩余发品额度: %d", quota)
	}
}

func (h *SpuLimitHandler) Handle(ctx *model.TaskContext) error {
	spuLimitCount, err := ctx.OtherAPI.GetSpuLimitCount()
	if err != nil {
		logrus.Infof("获取发品额度失败: %v\n", err)
		return err
	}

	// 输出日志信息
	logrus.Infof("发品额度信息: 可用状态=%d, 可用额度=%d, 已使用额度=%d, 剩余额度=%d\n",
		spuLimitCount.AbleStatus,
		spuLimitCount.QuotaAvailable,
		spuLimitCount.QuotaUsed,
		spuLimitCount.QuotaRemain)

	// 提交剩余发品额度到管理系统
	if ctx.MemoryManager != nil && ctx.MemoryManager.DailyCountManager != nil {
		h.submitRemainingQuota(ctx, spuLimitCount.QuotaRemain)
	}

	// 检查发品额度是否充足
	if spuLimitCount.QuotaRemain < 1 {
		logrus.Warnf("发品额度不足，终止任务: 可用状态=%d, 剩余额度=%d",
			spuLimitCount.AbleStatus, spuLimitCount.QuotaRemain)
		return model.NewFilteredError("发品额度不足")
	}

	ctx.SpuLimitCount = spuLimitCount
	return nil
}
