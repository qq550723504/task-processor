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
	h.Logger().Infof("开始执行处理器: %s", h.Name())
	defer recovery.Recover(h.Name(), h.Logger())

	for _, v := range h.validators {
		h.Logger().Infof("执行验证器: %s", v.Name())
		if err := v.Validate(ctx); err != nil {
			return fmt.Errorf("验证失败 [%s]: %w", v.Name(), err)
		}
		h.Logger().Infof("验证器通过: %s", v.Name())
	}

	ctx.SetData("validation_passed", true)
	ctx.SetData("validation_count", len(h.validators))

	h.Logger().Infof("处理器执行成功: %s", h.Name())
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
