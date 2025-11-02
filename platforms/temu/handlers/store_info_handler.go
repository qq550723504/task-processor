package handlers

import (
	"fmt"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// StoreInfoHandler 店铺信息处理器
type StoreInfoHandler struct {
	logger      *logrus.Entry
	storeClient interface {
		GetStore(id int64) (*api.StoreRespDTO, error)
	}
}

// NewStoreInfoHandler 创建新的店铺信息处理器
func NewStoreInfoHandler(storeClient interface {
	GetStore(id int64) (*api.StoreRespDTO, error)
}) *StoreInfoHandler {
	return &StoreInfoHandler{
		logger:      logrus.WithField("handler", "StoreInfoHandler"),
		storeClient: storeClient,
	}
}

// Name 返回处理器名称
func (h *StoreInfoHandler) Name() string {
	return "店铺信息处理器"
}

// Handle 处理任务
func (h *StoreInfoHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始获取店铺信息")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.StoreID == 0 {
		return fmt.Errorf("店铺ID为空")
	}

	// 获取店铺信息
	storeInfo, err := h.storeClient.GetStore(ctx.Task.StoreID)
	if err != nil {
		h.logger.Errorf("获取店铺信息失败: %v", err)
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	if storeInfo == nil {
		return fmt.Errorf("店铺信息为空")
	}

	// 将店铺信息存储到上下文中
	ctx.StoreInfo = storeInfo

	h.logger.Infof("成功获取店铺信息: StoreID=%d, StoreName=%s",
		storeInfo.ID, storeInfo.Name)
	return nil
}
