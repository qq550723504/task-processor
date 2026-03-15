// Package shein 提供SHEIN平台的处理器适配器
package pipeline

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/shein/model"
)

// SheinHandlerAdapter SHEIN处理器适配器，将SHEIN的StepHandler适配到通用的Handler接口
type SheinHandlerAdapter struct {
	handler model.StepHandler
}

// NewSheinHandlerAdapter 创建新的SHEIN处理器适配器
func NewSheinHandlerAdapter(handler model.StepHandler) *SheinHandlerAdapter {
	return &SheinHandlerAdapter{
		handler: handler,
	}
}

// Name 返回处理器名称
func (a *SheinHandlerAdapter) Name() string {
	return a.handler.Name()
}

// Handle 处理任务，将通用TaskContext转换为SHEIN的TaskContext
func (a *SheinHandlerAdapter) Handle(ctx pipeline.TaskContext) error {
	// 这里需要将通用的TaskContext转换为SHEIN的TaskContext
	// 由于SHEIN的TaskContext是具体类型，我们需要进行类型转换
	if sheinCtx, ok := ctx.(*model.TaskContext); ok {
		return a.handler.Handle(sheinCtx)
	}

	// 如果类型转换失败，返回错误
	return fmt.Errorf("无法将TaskContext转换为SHEIN TaskContext，实际类型: %T", ctx)
}
