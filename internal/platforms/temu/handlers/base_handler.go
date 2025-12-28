// Package handlers 提供TEMU处理器基类
package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// BaseTemuHandler TEMU处理器基类，封装类型断言逻辑
type BaseTemuHandler struct {
	logger *logrus.Entry
	name   string
}

// NewBaseTemuHandler 创建TEMU处理器基类
func NewBaseTemuHandler(name string) *BaseTemuHandler {
	return &BaseTemuHandler{
		logger: logrus.WithField("handler", name),
		name:   name,
	}
}

// Name 返回处理器名称
func (h *BaseTemuHandler) Name() string {
	return h.name
}

// GetTemuContext 获取强类型TEMU上下文，封装类型断言逻辑
func (h *BaseTemuHandler) GetTemuContext(ctx pipeline.TaskContext) (*temucontext.TemuTaskContext, error) {
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return nil, fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
	}
	return temuCtx, nil
}

// TemuHandlerFunc TEMU处理器函数类型
type TemuHandlerFunc func(*temucontext.TemuTaskContext) error

// HandleWithTemuContext 使用强类型上下文处理任务的通用方法
func (h *BaseTemuHandler) HandleWithTemuContext(ctx pipeline.TaskContext, handlerFunc TemuHandlerFunc) error {
	temuCtx, err := h.GetTemuContext(ctx)
	if err != nil {
		return err
	}

	return handlerFunc(temuCtx)
}
