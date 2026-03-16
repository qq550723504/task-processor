package store

import (
	"fmt"
	"task-processor/internal/shein"
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

func (h *SupplierInfoHandler) Handle(ctx *shein.TaskContext) error {
	// 检查 OtherAPI 是否为 nil
	if ctx.OtherAPI == nil {
		return shein.NewRetryableError("OtherAPI客户端未初始化", nil)
	}

	soi, err := ctx.OtherAPI.GetSupplierOperateInfo()
	if err != nil {
		return shein.NewRetryableError(fmt.Sprintf("获取供应商操作信息失败: %v", err), err)
	}

	ctx.SupplierInfo = &soi.Info
	return nil
}
