package info

import (
	"fmt"
	"os"
	"strconv"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// StoreIDHandler 处理店铺StoreID
type StoreIDHandler struct {
	storeClient interface {
		UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error)
		UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error)
	}
}

func NewStoreIDHandler(storeClient interface {
	UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error)
	UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error)
}) *StoreIDHandler {
	return &StoreIDHandler{
		storeClient: storeClient,
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

func (h *StoreIDHandler) Handle(ctx *model.TaskContext) error {
	// 直接调用店铺客户端的UpdateStoreId方法
	storeID := ctx.SupplierInfo.StoreID
	if ctx.StoreInfo != nil && ctx.StoreInfo.StoreID == "" {
		req := &api.StoreIdUpdateReqDTO{
			ID:      ctx.StoreInfo.ID,
			StoreID: fmt.Sprintf("%d", storeID),
		}
		_, err := h.storeClient.UpdateStoreId(req)
		if err != nil {
			logrus.Infof("[实例%s] 处理店铺StoreID失败: %v\n", GetInstanceID(), err)
			return err
		}
	}

	// 添加逻辑：如果StoreInfo不为nil，并且StoreID与ctx.SupplierInfo.Info.StoreID这里的ID不一致时就禁用店铺
	if ctx.StoreInfo != nil {
		// 检查StoreID是否为空
		if ctx.StoreInfo.StoreID == "" {
			logrus.Infof("[实例%s] StoreInfo.StoreID为空，跳过店铺状态检查\n", GetInstanceID())
		} else {
			// 将ctx.StoreInfo.StoreID转换为int64进行比较
			storeInfoStoreID, err := strconv.ParseInt(ctx.StoreInfo.StoreID, 10, 64)
			if err != nil {
				logrus.Infof("[实例%s] 解析StoreInfo.StoreID失败: %v\n", GetInstanceID(), err)
				return err
			}

			// 比较StoreID是否不一致
			if storeInfoStoreID != ctx.SupplierInfo.StoreID {
				// 禁用店铺
				statusReq := &api.StoreStatusUpdateReqDTO{
					ID:     ctx.StoreInfo.ID,
					Status: 1, // 1表示禁用
				}
				_, err := h.storeClient.UpdateStoreStatus(statusReq)
				if err != nil {
					logrus.Infof("[实例%s] 更新店铺状态失败: %v\n", GetInstanceID(), err)
					return err
				}
				logrus.Infof("[实例%s] 店铺ID不一致，已禁用店铺: StoreInfo.StoreID=%d, SupplierInfo.StoreID=%d\n",
					GetInstanceID(), storeInfoStoreID, ctx.SupplierInfo.StoreID)
			}
		}
	}

	return nil
}
