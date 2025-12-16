// Package handler 提供店铺信息处理器实现
package handler

import (
	"fmt"
	"task-processor/platforms/amazon/internal/model"
)

// StoreInfoHandler 店铺信息处理器
type StoreInfoHandler struct {
	*BaseHandler
}

// NewStoreInfoHandler 创建店铺信息处理器
func NewStoreInfoHandler() *StoreInfoHandler {
	return &StoreInfoHandler{
		BaseHandler: NewBaseHandler("获取店铺信息"),
	}
}

// Execute 执行店铺信息处理
func (h *StoreInfoHandler) Execute(services *model.Services, data map[string]interface{}) error {
	h.GetLogger().Info("开始获取店铺信息")

	// 验证服务
	if err := h.ValidateServices(services); err != nil {
		return err
	}

	if services.ManagementClient == nil {
		return fmt.Errorf("管理客户端未初始化")
	}

	// 获取店铺ID
	storeID, err := h.GetRequiredInt64(data, "store_id")
	if err != nil {
		return err
	}

	// 获取店铺信息
	storeClient := services.ManagementClient.GetStoreClient()
	storeInfo, err := storeClient.GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 保存店铺信息到数据中
	h.SetResult(data, "store_info", storeInfo)

	h.GetLogger().Infof("店铺信息获取成功: %s", storeInfo.Name)
	return nil
}
