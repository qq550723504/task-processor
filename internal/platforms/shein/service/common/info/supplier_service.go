package info

import (
	"fmt"
	"task-processor/internal/platforms/shein/model"
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

func (h *SupplierInfoHandler) Handle(ctx *model.TaskContext) error {
	soi, err := ctx.ShopClient.GetSupplierOperateInfo()
	if err != nil {
		return model.NewRetryableError(fmt.Sprintf("获取供应商操作信息失败: %v", err), err)
	}

	ctx.SupplierInfo = &soi.Info
	return nil
}
