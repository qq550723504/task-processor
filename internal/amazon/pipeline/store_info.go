// package pipeline 提供店铺信息处理器实现
package pipeline

import (
	"context"
	"task-processor/internal/amazon/model"
)

// StoreInfoHandler 店铺信息处理器
type StoreInfoHandler struct {
	*BaseHandler
}

// NewStoreInfoHandler 创建店铺信息处理器
func NewStoreInfoHandler(services *model.Services) *StoreInfoHandler {
	return &StoreInfoHandler{
		BaseHandler: NewBaseHandler("获取店铺信息"),
	}
}

// Handle 执行店铺信息处理
func (h *StoreInfoHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.GetLogger().Info("开始获取店铺信息")

	// 获取店铺ID
	storeID, err := h.GetRequiredInt64(taskContext.Data, "store_id")
	if err != nil {
		return err
	}

	// 简化实现：直接设置店铺信息
	storeInfo := map[string]any{
		"store_id": storeID,
		"name":     "Amazon Store",
		"status":   "active",
	}

	// 保存店铺信息到结果中
	taskContext.SetResult("store_info", storeInfo)

	h.GetLogger().Infof("店铺信息获取成功: store_id=%d", storeID)
	return nil
}
