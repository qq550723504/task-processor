// Package handlers 提供公共处理器实现
package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/recovery"
)

// ValidationHandler 通用验证处理器
type ValidationHandler struct {
	*pipeline.BaseHandler
	validators []Validator
}

// Validator 验证器接口
type Validator interface {
	Validate(ctx pipeline.TaskContext) error
	Name() string
}

// NewValidationHandler 创建验证处理器
func NewValidationHandler(validators ...Validator) pipeline.Handler {
	return &ValidationHandler{
		BaseHandler: pipeline.NewBaseHandler("通用验证处理器"),
		validators:  validators,
	}
}

// Handle 执行验证处理
func (h *ValidationHandler) Handle(ctx pipeline.TaskContext) error {
	h.LogStart()
	defer recovery.Recover("验证处理器", h.GetLogger())

	// 验证上下文
	if err := h.ValidateContext(ctx); err != nil {
		h.LogError(err)
		return err
	}

	// 执行所有验证器
	for _, validator := range h.validators {
		h.GetLogger().Infof("执行验证器: %s", validator.Name())

		if err := validator.Validate(ctx); err != nil {
			validationErr := fmt.Errorf("验证失败 [%s]: %w", validator.Name(), err)
			h.LogError(validationErr)
			return validationErr
		}

		h.GetLogger().Infof("验证器通过: %s", validator.Name())
	}

	// 设置验证结果
	h.SetResult(ctx, "validation_passed", true)
	h.SetResult(ctx, "validation_count", len(h.validators))

	h.LogSuccess()
	return nil
}

// TaskValidator 任务基础验证器
type TaskValidator struct{}

// NewTaskValidator 创建任务验证器
func NewTaskValidator() Validator {
	return &TaskValidator{}
}

// Name 返回验证器名称
func (v *TaskValidator) Name() string {
	return "任务基础验证器"
}

// Validate 执行任务验证
func (v *TaskValidator) Validate(ctx pipeline.TaskContext) error {
	task := ctx.GetTask()

	if task.ProductID == "" {
		return fmt.Errorf("产品ID不能为空")
	}

	if task.StoreID <= 0 {
		return fmt.Errorf("店铺ID无效: %d", task.StoreID)
	}

	if task.Platform == "" {
		return fmt.Errorf("平台信息不能为空")
	}

	return nil
}
