package handlers

import (
	"task-processor/platforms/amazon"

	"github.com/sirupsen/logrus"
)

// ProductDataHandler 产品数据处理器
type ProductDataHandler struct{}

// NewProductDataHandler 创建产品数据处理器
func NewProductDataHandler() *ProductDataHandler {
	return &ProductDataHandler{}
}

// Name 返回处理器名称
func (h *ProductDataHandler) Name() string {
	return "获取产品数据"
}

// Handle 处理逻辑
func (h *ProductDataHandler) Handle(ctx *amazon.TaskContext) error {
	productID := ctx.Task.ProductID

	logrus.WithFields(logrus.Fields{
		"productID": productID,
	}).Info("开始获取产品数据")

	// TODO: 实现实际的产品数据获取逻辑
	// 这里需要根据实际的管理系统 API 进行调用

	// 保存到上下文
	ctx.SetData("product_id", productID)

	logrus.Info("产品数据获取成功")
	return nil
}
