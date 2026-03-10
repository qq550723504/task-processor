// Package handler 提供验证处理器实现
package handler

import (
	"context"
	"fmt"
	"task-processor/internal/platforms/amazon/core/model"
)

// ValidationHandler 验证处理器
type ValidationHandler struct {
	*BaseHandler
}

// NewValidationHandler 创建验证处理器
func NewValidationHandler(services *model.Services) *ValidationHandler {
	return &ValidationHandler{
		BaseHandler: NewBaseHandler("产品数据验证"),
	}
}

// Handle 执行产品数据验证
func (h *ValidationHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始验证产品数据")

	// 验证必要字段
	if err := h.validateRequiredFields(taskContext.Data); err != nil {
		return fmt.Errorf("必要字段验证失败: %w", err)
	}

	// 简化验证：只要有原始JSON数据就通过
	if _, exists := taskContext.Data["raw_json_data"]; !exists {
		return fmt.Errorf("缺少原始JSON数据")
	}

	h.logger.Info("产品数据验证通过")
	return nil
}

// validateRequiredFields 验证必要字段
func (h *ValidationHandler) validateRequiredFields(data map[string]interface{}) error {
	requiredFields := []string{
		"product_id",
		"store_id",
		"tenant_id",
	}

	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			return fmt.Errorf("缺少必要字段: %s", field)
		}
	}

	return nil
}
