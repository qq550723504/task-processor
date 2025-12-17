package modules

import (
	"time"

	management_api "task-processor/internal/common/management/api"

	"github.com/sirupsen/logrus"
)

// StoreInfoHandler 获取店铺信息处理器
type StoreInfoHandler struct {
	storeClient interface {
		GetStore(id int64) (*management_api.StoreRespDTO, error)
	}
}

func NewStoreInfoHandler(storeClient interface {
	GetStore(id int64) (*management_api.StoreRespDTO, error)
}) *StoreInfoHandler {
	return &StoreInfoHandler{storeClient: storeClient}
}

func (h *StoreInfoHandler) Name() string {
	return "获取店铺信息"
}

func (h *StoreInfoHandler) Handle(ctx *TaskContext) error {
	// 如果是Amazon任务，跳过Shein店铺信息获取
	if ShouldSkipForAmazon(ctx) {
		logrus.Infof("[StoreInfo] Amazon任务，跳过店铺信息获取")
		return nil
	}

	storeInfo, err := h.storeClient.GetStore(ctx.Task.StoreID)
	if err != nil {
		// 检查原始错误是否已经是不可重试的
		if retryableErr, ok := err.(interface{ IsRetryable() bool }); ok {
			if !retryableErr.IsRetryable() {
				// 如果原始错误已经标记为不可重试，直接返回不可重试错误
				return NewNonRetryableError("获取店铺基础信息失败", err)
			}
		}
		// 网络错误或临时性错误可重试，数据错误不可重试
		return NewRetryableError("获取店铺基础信息失败", err)
	}

	if !*storeInfo.EnableAutoListing {
		// 暂停店铺上架并清理相关缓存
		if ctx.ShopClientMgr != nil {
			ctx.ShopClientMgr.RemoveClient(ctx.Task.TenantID, ctx.Task.StoreID)
			// 记录日志
			logrus.Infof("已删除店铺 %d:%d 的客户端缓存", ctx.Task.TenantID, ctx.Task.StoreID)
		}

		// 设置暂停键，暂停该店铺
		if ctx.MemoryManager != nil {
			ctx.MemoryManager.ShopPauseManager.PauseShop(
				ctx.Task.TenantID,
				ctx.Task.StoreID,
				"店铺未开启自动上架",
				24*time.Hour, // 暂停24小时
			)
			logrus.Infof("已暂停店铺 %d:%d 上架24小时，原因: 店铺未开启自动上架", ctx.Task.TenantID, ctx.Task.StoreID)
		}

		return NewNonRetryableError("店铺未开启自动上架", nil)
	}

	ctx.StoreInfo = storeInfo
	return nil
}
