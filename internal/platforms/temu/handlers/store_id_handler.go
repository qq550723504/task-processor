package handlers

import (
	"fmt"
	"os"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/management/api"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// StoreIDHandler 处理店铺StoreID检查和保存
type StoreIDHandler struct {
	logger      *logrus.Entry
	storeClient interface {
		UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error)
		UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error)
	}
}

// NewStoreIDHandler 创建新的店铺ID处理器
func NewStoreIDHandler(storeClient interface {
	UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error)
	UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error)
}) *StoreIDHandler {
	return &StoreIDHandler{
		logger:      logrus.WithField("handler", "StoreIDHandler"),
		storeClient: storeClient,
	}
}

// GetInstanceID 获取当前实例ID
func GetInstanceID() string {
	hostname, _ := os.Hostname()
	return hostname
}

// Name 返回处理器名称
func (h *StoreIDHandler) Name() string {
	return "店铺ID检查和保存处理器"
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *StoreIDHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *StoreIDHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始处理店铺ID检查和保存")

	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if task.StoreID == 0 {
		return fmt.Errorf("任务中的店铺ID为空")
	}

	storeInfo := temuCtx.StoreInfo
	if storeInfo == nil {
		return fmt.Errorf("店铺信息为空，请确保已执行店铺信息获取步骤")
	}

	// 从APIClient的Cookie中获取MALL_ID
	apiClient := temuCtx.APIClient
	if apiClient == nil {
		return fmt.Errorf("APIClient为空，无法获取MALL_ID")
	}

	mallID := apiClient.GetMallID()
	if mallID == "" {
		h.logger.Warn("Cookie中未找到MALL_ID，可能需要重新登录")
		return fmt.Errorf("Cookie中未找到MALL_ID，请检查登录状态")
	}

	h.logger.Infof("从Cookie中获取到MALL_ID: %s", mallID)

	// 检查并更新店铺ID（如果StoreID为空）
	if storeInfo.StoreID == "" {
		h.logger.Infof("店铺ID为空，更新为Cookie中的MALL_ID: %s", mallID)

		req := &api.StoreIdUpdateReqDTO{
			ID:      storeInfo.ID,
			StoreID: mallID,
		}

		_, err := h.storeClient.UpdateStoreId(req)
		if err != nil {
			h.logger.Errorf("[实例%s] 更新店铺StoreID失败: %v", GetInstanceID(), err)
			return fmt.Errorf("更新店铺StoreID失败: %w", err)
		}

		h.logger.Infof("[实例%s] 成功更新店铺StoreID: %s", GetInstanceID(), mallID)
	} else {
		// 检查StoreID是否一致
		if storeInfo.StoreID != mallID {
			h.logger.Warnf("[实例%s] 店铺ID不一致: StoreInfo.StoreID=%s, Cookie.MALL_ID=%s",
				GetInstanceID(), storeInfo.StoreID, mallID)

			// 如果管理系统中的StoreID不为空，则更新Cookie中的MALL_ID
			if storeInfo.StoreID != "" {
				h.logger.Infof("[实例%s] 将Cookie中的MALL_ID从 %s 更新为管理系统中的StoreID: %s",
					GetInstanceID(), mallID, storeInfo.StoreID)

				apiClient.SetMallID(storeInfo.StoreID)
				h.logger.Infof("[实例%s] 成功更新Cookie中的MALL_ID为: %s", GetInstanceID(), storeInfo.StoreID)
			} else {
				// 如果管理系统中的StoreID为空，则禁用店铺
				h.logger.Warnf("[实例%s] 管理系统中的StoreID为空，禁用店铺", GetInstanceID())

				statusReq := &api.StoreStatusUpdateReqDTO{
					ID:     storeInfo.ID,
					Status: 1, // 1表示禁用
				}

				_, err := h.storeClient.UpdateStoreStatus(statusReq)
				if err != nil {
					h.logger.Errorf("[实例%s] 更新店铺状态失败: %v", GetInstanceID(), err)
					return fmt.Errorf("更新店铺状态失败: %w", err)
				}

				h.logger.Infof("[实例%s] 管理系统StoreID为空，已禁用店铺", GetInstanceID())
				return fmt.Errorf("管理系统StoreID为空，已禁用店铺")
			}
		} else {
			h.logger.Infof("[实例%s] 店铺ID验证通过: StoreID=%s", GetInstanceID(), mallID)
		}
	}

	h.logger.Info("✅ 店铺ID检查和保存处理完成")
	return nil
}
