// Package handler 提供验证处理器实现
package handler

import (
	"fmt"
	"task-processor/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// ValidationHandler 验证处理器
type ValidationHandler struct {
	logger *logrus.Entry
}

// NewValidationHandler 创建验证处理器
func NewValidationHandler() *ValidationHandler {
	return &ValidationHandler{
		logger: logrus.WithField("handler", "ValidationHandler"),
	}
}

// Name 返回处理器名称
func (h *ValidationHandler) Name() string {
	return "产品数据验证"
}

// Execute 执行产品数据验证
func (h *ValidationHandler) Execute(services *model.Services, data map[string]interface{}) error {
	h.logger.Info("开始验证产品数据")

	// 验证必要字段
	if err := h.validateRequiredFields(data); err != nil {
		return fmt.Errorf("必要字段验证失败: %w", err)
	}

	// 验证产品数据格式
	if err := h.validateProductData(data); err != nil {
		return fmt.Errorf("产品数据格式验证失败: %w", err)
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

// validateProductData 验证产品数据
func (h *ValidationHandler) validateProductData(data map[string]interface{}) error {
	rawData, exists := data["raw_product_data"]
	if !exists {
		return fmt.Errorf("缺少原始产品数据")
	}

	if rawData == nil {
		return fmt.Errorf("原始产品数据为空")
	}

	// 验证数据格式
	rawDataStr, ok := rawData.(string)
	if !ok {
		return fmt.Errorf("原始产品数据格式错误")
	}

	if len(rawDataStr) == 0 {
		return fmt.Errorf("原始产品数据内容为空")
	}

	return nil
}
