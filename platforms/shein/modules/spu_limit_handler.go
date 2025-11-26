package modules

import (
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
func (h *SpuLimitHandler) submitRemainingQuota(ctx *TaskContext, quota int) {
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

func (h *SpuLimitHandler) Handle(ctx *TaskContext) error {
	spuLimitCount, err := ctx.ShopClient.GetSpuLimitCount()
	if err != nil {
		logrus.Infof("获取发品额度失败: %v\n", err)
		return err
	}

	// if spuLimitCount.AbleStatus == 0 || spuLimitCount.QuotaAvailable < 1 {
	// 	logrus.Infof("发品额度不足: %+v\n", spuLimitCount)

	// 	// 注意：不再需要清空Redis队列中的待处理任务
	// 	// 因为任务现在通过API管理，没有本地队列
	// 	tenantID := fmt.Sprintf("%d", ctx.Task.TenantID)
	// 	shopID := fmt.Sprintf("%d", ctx.Task.StoreID)
	// 	logrus.Infof("店铺 %s:%s 发品额度不足，任务将通过API重新调度", tenantID, shopID)

	// 	// 返回不可重试错误，防止任务重新入队
	// 	return NewNonRetryableError("发品额度不足",
	// 		fmt.Errorf("发品额度不足: %d", spuLimitCount.QuotaAvailable))
	// }

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

	ctx.SpuLimitCount = spuLimitCount
	return nil
}
