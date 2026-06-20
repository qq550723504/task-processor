package store

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"task-processor/internal/core/logger"
	"task-processor/internal/listingadmin"
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// StoreIDHandler 处理店铺StoreID
type StoreIDHandler struct {
	logger    *logrus.Entry
	storeRepo interface {
		UpdateStoreID(ctx context.Context, id int64, storeID string) (*listingadmin.Store, error)
		UpdateStoreStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*listingadmin.Store, error)
	}
}

func NewStoreIDHandler(storeRepo interface {
	UpdateStoreID(ctx context.Context, id int64, storeID string) (*listingadmin.Store, error)
	UpdateStoreStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*listingadmin.Store, error)
}) *StoreIDHandler {
	return &StoreIDHandler{
		logger:    logger.GetGlobalLogger("store_id_handler"),
		storeRepo: storeRepo,
	}
}

// GetInstanceID 获取当前实例ID
func GetInstanceID() string {
	hostname, _ := os.Hostname()
	return hostname
}

func (h *StoreIDHandler) Name() string {
	return "处理店铺StoreID"

}

func (h *StoreIDHandler) Handle(ctx *shein.TaskContext) error {
	if h.storeRepo == nil {
		return fmt.Errorf("store repository is nil")
	}
	if ctx == nil || ctx.StoreInfo == nil || ctx.SupplierInfo == nil {
		return fmt.Errorf("store info or supplier info is nil")
	}
	repoCtx := context.Background()
	if ctx.Context != nil {
		repoCtx = ctx.Context
	}

	// 直接调用店铺客户端的UpdateStoreId方法
	storeID := ctx.SupplierInfo.StoreID
	if ctx.StoreInfo != nil && ctx.StoreInfo.StoreID == "" {
		_, err := h.storeRepo.UpdateStoreID(repoCtx, ctx.StoreInfo.ID, fmt.Sprintf("%d", storeID))
		if err != nil {
			h.logger.Infof("[实例%s] 处理店铺StoreID失败: %v", GetInstanceID(), err)
			return err
		}
	}

	// 添加逻辑：如果StoreInfo不为nil，并且StoreID与ctx.SupplierInfo.Info.StoreID这里的ID不一致时就禁用店铺
	if ctx.StoreInfo != nil {
		// 检查StoreID是否为空
		if ctx.StoreInfo.StoreID == "" {
			h.logger.Infof("[实例%s] StoreInfo.StoreID为空，跳过店铺状态检查", GetInstanceID())
		} else {
			// 将ctx.StoreInfo.StoreID转换为int64进行比较
			storeInfoStoreID, err := strconv.ParseInt(ctx.StoreInfo.StoreID, 10, 64)
			if err != nil {
				h.logger.Infof("[实例%s] 解析StoreInfo.StoreID失败: %v", GetInstanceID(), err)
				return err
			}

			// 比较StoreID是否不一致
			if storeInfoStoreID != ctx.SupplierInfo.StoreID {
				// 禁用店铺
				tenantID := int64(0)
				if ctx.Task != nil {
					tenantID = ctx.Task.TenantID
				}
				_, err := h.storeRepo.UpdateStoreStatus(repoCtx, tenantID, ctx.StoreInfo.ID, 1, "SHEIN supplier store id mismatch")
				if err != nil {
					h.logger.Infof("[实例%s] 更新店铺状态失败: %v", GetInstanceID(), err)
					return err
				}
				h.logger.Infof("[实例%s] 店铺ID不一致，已禁用店铺: StoreInfo.StoreID=%d, SupplierInfo.StoreID=%d",
					GetInstanceID(), storeInfoStoreID, ctx.SupplierInfo.StoreID)
			}
		}
	}

	return nil
}
