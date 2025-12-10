// Package handlers 提供Amazon平台处理步骤
package handlers

import (
	"task-processor/platforms/amazon"

	"github.com/sirupsen/logrus"
)

// StoreInfoHandler 店铺信息处理器
type StoreInfoHandler struct{}

// NewStoreInfoHandler 创建店铺信息处理器
func NewStoreInfoHandler() *StoreInfoHandler {
	return &StoreInfoHandler{}
}

// Name 返回处理器名称
func (h *StoreInfoHandler) Name() string {
	return "获取店铺信息"
}

// Handle 处理逻辑
func (h *StoreInfoHandler) Handle(ctx *amazon.TaskContext) error {
	storeID := ctx.Task.StoreID

	logrus.WithFields(logrus.Fields{
		"storeID": storeID,
	}).Info("开始获取店铺信息")

	// TODO: 实现实际的店铺信息获取逻辑
	// 这里需要根据实际的管理系统 API 进行调用

	// 保存到上下文
	ctx.SetData("store_id", storeID)

	logrus.Info("店铺信息获取成功")
	return nil
}
