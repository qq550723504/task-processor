package modules

import (
	"fmt"
)

// SupplierInfoHandler 获取供应商信息处理器
type SupplierInfoHandler struct {
	storeClient interface {
		DeleteStoreCookie(id int64) (bool, error)
	}
}

func NewSupplierInfoHandler(storeClient interface {
	DeleteStoreCookie(id int64) (bool, error)
}) *SupplierInfoHandler {
	return &SupplierInfoHandler{
		storeClient: storeClient,
	}
}

func (h *SupplierInfoHandler) Name() string {
	return "获取供应商操作信息"
}

func (h *SupplierInfoHandler) Handle(ctx *TaskContext) error {
	soi, err := ctx.ShopClient.GetSupplierOperateInfo()
	if err != nil {
		return NewRetryableError(fmt.Sprintf("获取供应商操作信息失败: %v", err), err)
	}

	// 检查API调用是否成功
	if soi.Code != "0" {
		// 检查是否是认证过期错误（20302）
		if soi.Code == "20302" {
			// 删除过期的Cookie
			if h.storeClient != nil {
				_, err := h.storeClient.DeleteStoreCookie(ctx.Task.StoreID)
				if err != nil {
					return fmt.Errorf("删除过期Cookie失败: %v, 原始错误: code=%s, msg=%s", err, soi.Code, soi.Msg)
				}
			}
			return NewNonRetryableError(fmt.Sprintf("认证已过期，Cookie已删除，请重新登录: code=%s, msg=%s", soi.Code, soi.Msg), nil)
		}
		return fmt.Errorf("获取供应商操作信息失败: code=%s, msg=%s", soi.Code, soi.Msg)
	}

	ctx.SupplierInfo = &soi.Info
	return nil
}
