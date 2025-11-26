package modules

import (
	shops "task-processor/common/shein"
	"task-processor/common/shein/api"

	"github.com/sirupsen/logrus"
)

// ShopClientHandler 获取店铺API客户端处理器
type ShopClientHandler struct {
	shopClientMgr    *shops.ClientManager
	managementClient interface {
		GetStoreCookie(id int64) (string, error)
	}
	cookieManager interface {
		SetCookie(tenantID, shopID int64, cookie string)
	}
}

func NewShopClientHandler(
	shopClientMgr *shops.ClientManager,
	managementClient interface {
		GetStoreCookie(id int64) (string, error)
	},
	cookieManager interface {
		SetCookie(tenantID, shopID int64, cookie string)
	},
) *ShopClientHandler {
	return &ShopClientHandler{
		shopClientMgr:    shopClientMgr,
		managementClient: managementClient,
		cookieManager:    cookieManager,
	}
}

func (h *ShopClientHandler) Name() string {
	return "获取店铺API客户端"
}

func (h *ShopClientHandler) Handle(ctx *TaskContext) error {
	shopClient, err := h.shopClientMgr.GetClient(ctx.Task.TenantID, ctx.Task.StoreID, ctx.StoreInfo)
	if err != nil {
		logrus.Debugf("获取客户端失败，错误类型: %T, 错误: %v", err, err)

		// 检查是否是 Cookie 不存在的错误
		cookieErr, ok := err.(*api.CookieError)
		logrus.Debugf("类型断言结果: ok=%v, cookieErr=%+v", ok, cookieErr)

		if ok && cookieErr.Code == "COOKIE_NOT_FOUND" {
			// 尝试从 API 获取 Cookie
			logrus.Infof("从API获取Cookie: 租户=%d, 店铺=%d", ctx.Task.TenantID, ctx.Task.StoreID)

			cookie, err := h.managementClient.GetStoreCookie(ctx.Task.StoreID)
			if err != nil {
				// 获取Cookie失败，使用ShopPauseManager暂停店铺（认证过期类型）
				logrus.Warnf("从API获取Cookie失败，将暂停店铺 %d: %v", ctx.Task.StoreID, err)

				if ctx.MemoryManager != nil && ctx.MemoryManager.ShopPauseManager != nil {
					ctx.MemoryManager.ShopPauseManager.PauseShopForAuthExpired(
						ctx.Task.TenantID,
						ctx.Task.StoreID,
						"Cookie获取失败，需要重新登录",
					)
				} else {
					logrus.Warn("MemoryManager未初始化，无法设置暂停状态")
				}

				return NewRetryableError("从API获取Cookie失败，店铺已暂停，等待恢复后重试", err)
			}

			// 保存到内存
			h.cookieManager.SetCookie(ctx.Task.TenantID, ctx.Task.StoreID, cookie)
			logrus.Infof("Cookie已保存到内存: 租户=%d, 店铺=%d", ctx.Task.TenantID, ctx.Task.StoreID)

			// 重新尝试获取客户端
			shopClient, err = h.shopClientMgr.GetClient(ctx.Task.TenantID, ctx.Task.StoreID, ctx.StoreInfo)
			if err != nil {
				return NewRetryableError("获取店铺API客户端失败", err)
			}
		} else {
			// 其他错误
			logrus.Warnf("非Cookie错误或类型断言失败: %v", err)
			return NewRetryableError("获取店铺API客户端失败", err)
		}
	}

	ctx.ShopClient = shopClient
	return nil
}
