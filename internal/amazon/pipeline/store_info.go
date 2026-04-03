// package pipeline 提供店铺信息处理器实现
package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/model"
)

// StoreInfoHandler 店铺信息处理器
type StoreInfoHandler struct {
	*BaseHandler
	services *model.Services
}

// NewStoreInfoHandler 创建店铺信息处理器
func NewStoreInfoHandler(services *model.Services) *StoreInfoHandler {
	return &StoreInfoHandler{
		BaseHandler: NewBaseHandler("获取店铺信息", amazonTaskStageInitStore),
		services:    services,
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

	if h.services == nil || h.services.ManagementClient == nil {
		h.GetLogger().Warn("管理客户端未初始化，使用兜底店铺信息")
		storeInfo := map[string]any{
			"store_id": storeID,
			"name":     "Amazon Store",
			"status":   "active",
		}
		taskContext.SetResult("store_info", storeInfo)
		return nil
	}
	storeClient := h.services.ManagementClient.GetStoreClient()
	if storeClient == nil {
		h.GetLogger().Warn("店铺客户端未初始化，使用兜底店铺信息")
		storeInfo := map[string]any{
			"store_id": storeID,
			"name":     "Amazon Store",
			"status":   "active",
		}
		taskContext.SetResult("store_info", storeInfo)
		return nil
	}
	storeResp, err := storeClient.GetStore(storeID)
	if err != nil {
		return err
	}
	if storeResp == nil {
		return fmt.Errorf("店铺不存在: %d", storeID)
	}
	taskContext.StoreInfo = storeResp

	storeInfo := map[string]any{
		"store_id":    storeID,
		"name":        storeResp.Name,
		"status":      storeResp.Status,
		"platform":    storeResp.Platform,
		"daily_limit": storeResp.DailyLimit,
	}

	// 保存店铺信息到结果中
	taskContext.SetResult("store_info", storeInfo)

	h.GetLogger().Infof("店铺信息获取成功: store_id=%d", storeID)
	return nil
}
