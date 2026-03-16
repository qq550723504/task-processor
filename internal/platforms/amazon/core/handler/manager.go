// Package handler 提供Amazon处理器管理功能
package handler

import (
	"context"
	"task-processor/internal/platforms/amazon/core/model"

	"github.com/sirupsen/logrus"
)

// HandlerManager 处理器管理器
type HandlerManager struct {
	services *model.Services
	handlers []Handler
	logger   *logrus.Entry
}

// NewHandlerManager 创建处理器管理器
func NewHandlerManager(services *model.Services) *HandlerManager {
	manager := &HandlerManager{
		services: services,
		logger:   logrus.WithField("component", "HandlerManager"),
	}

	// 初始化所有处理器
	manager.initializeHandlers()

	return manager
}

// initializeHandlers 初始化所有处理器
func (m *HandlerManager) initializeHandlers() {
	// 按照处理顺序添加处理器
	m.handlers = []Handler{
		NewDataParserHandler(m.services),
		NewValidationHandler(m.services),
		NewStoreInfoHandler(m.services),
		NewProductTypeHandler(m.services),
		NewAttributeMapperHandler(m.services),
		NewProductDataHandler(m.services),
		NewImageHandler(m.services),
		NewVariantHandler(m.services),
		NewListingHandler(m.services),
		NewPricingHandler(m.services),
		NewInventoryHandler(m.services),
	}

	m.logger.WithField("handler_count", len(m.handlers)).Info("处理器初始化完成")
}

// ProcessProduct 处理产品
func (m *HandlerManager) ProcessProduct(ctx context.Context, taskContext *model.TaskContext) error {
	m.logger.WithField("task_id", taskContext.TaskID).Info("开始处理产品")

	// 依次执行所有处理器
	for i, handler := range m.handlers {
		m.logger.WithFields(logrus.Fields{
			"task_id":      taskContext.TaskID,
			"handler_step": i + 1,
			"handler_name": handler.Name(),
		}).Info("执行处理器")

		if err := handler.Handle(ctx, taskContext); err != nil {
			m.logger.WithError(err).WithFields(logrus.Fields{
				"task_id":      taskContext.TaskID,
				"handler_step": i + 1,
				"handler_name": handler.Name(),
			}).Error("处理器执行失败")
			return err
		}

		m.logger.WithFields(logrus.Fields{
			"task_id":      taskContext.TaskID,
			"handler_step": i + 1,
			"handler_name": handler.Name(),
		}).Info("处理器执行成功")
	}

	m.logger.WithField("task_id", taskContext.TaskID).Info("产品处理完成")
	return nil
}

// GetStatus 获取管理器状态
func (m *HandlerManager) GetStatus() map[string]any {
	handlerStatus := make([]map[string]any, len(m.handlers))

	for i, handler := range m.handlers {
		handlerStatus[i] = map[string]any{
			"name":   handler.Name(),
			"status": "ready",
		}
	}

	return map[string]any{
		"total_handlers": len(m.handlers),
		"handlers":       handlerStatus,
	}
}

// GetHandlerCount 获取处理器数量
func (m *HandlerManager) GetHandlerCount() int {
	return len(m.handlers)
}

// GetHandlerNames 获取所有处理器名称
func (m *HandlerManager) GetHandlerNames() []string {
	names := make([]string, len(m.handlers))
	for i, handler := range m.handlers {
		names[i] = handler.Name()
	}
	return names
}
