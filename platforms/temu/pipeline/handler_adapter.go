// Package pipeline TEMU处理器适配器，用于兼容现有处理器
package pipeline

import (
	"fmt"
	commonPipeline "task-processor/common/pipeline"
	"task-processor/platforms/temu/context"
)

// TemuHandlerInterface TEMU处理器接口（保持向后兼容）
type TemuHandlerInterface interface {
	Name() string
	Handle(ctx *context.TemuTaskContext) error
}

// LegacyHandlerInterface 旧版处理器接口（兼容现有处理器）
type LegacyHandlerInterface interface {
	Name() string
	Handle(ctx interface{}) error // 使用interface{}来兼容不同的上下文类型
}

// TemuHandlerAdapter TEMU处理器适配器
type TemuHandlerAdapter struct {
	handler TemuHandlerInterface
}

// NewTemuHandlerAdapter 创建TEMU处理器适配器
func NewTemuHandlerAdapter(handler interface{}) *TemuHandlerAdapter {
	// 检查是否实现了新接口
	if temuHandler, ok := handler.(TemuHandlerInterface); ok {
		return &TemuHandlerAdapter{
			handler: temuHandler,
		}
	}

	// 检查是否实现了旧接口
	if legacyHandler, ok := handler.(LegacyHandlerInterface); ok {
		return &TemuHandlerAdapter{
			handler: &LegacyHandlerWrapper{legacyHandler: legacyHandler},
		}
	}

	// 如果都不匹配，尝试反射调用（最后的兼容性保障）
	return &TemuHandlerAdapter{
		handler: &ReflectHandlerWrapper{handler: handler},
	}
}

// Name 获取处理器名称
func (tha *TemuHandlerAdapter) Name() string {
	return tha.handler.Name()
}

// Handle 处理任务（适配通用接口）
func (tha *TemuHandlerAdapter) Handle(ctx commonPipeline.TaskContextInterface) error {
	// 类型断言，确保是TEMU上下文
	temuCtx, ok := ctx.(*context.TemuTaskContext)
	if !ok {
		return fmt.Errorf("期望TemuTaskContext类型，实际得到: %T", ctx)
	}

	return tha.handler.Handle(temuCtx)
}

// LegacyHandlerWrapper 旧版处理器包装器
type LegacyHandlerWrapper struct {
	legacyHandler LegacyHandlerInterface
}

// Name 获取处理器名称
func (lhw *LegacyHandlerWrapper) Name() string {
	return lhw.legacyHandler.Name()
}

// Handle 处理任务（转换上下文类型）
func (lhw *LegacyHandlerWrapper) Handle(ctx *context.TemuTaskContext) error {
	// 这里需要将TemuTaskContext转换为旧的格式
	// 由于现有处理器期望的是旧的TaskContext，我们需要创建一个兼容的结构
	return lhw.legacyHandler.Handle(ctx)
}

// ReflectHandlerWrapper 反射处理器包装器（最后的兼容性保障）
type ReflectHandlerWrapper struct {
	handler interface{}
}

// Name 获取处理器名称
func (rhw *ReflectHandlerWrapper) Name() string {
	// 使用反射调用Name方法
	if namer, ok := rhw.handler.(interface{ Name() string }); ok {
		return namer.Name()
	}
	return "UnknownHandler"
}

// Handle 处理任务（使用反射调用）
func (rhw *ReflectHandlerWrapper) Handle(ctx *context.TemuTaskContext) error {
	// 使用反射调用Handle方法
	if handler, ok := rhw.handler.(interface{ Handle(interface{}) error }); ok {
		return handler.Handle(ctx)
	}
	return fmt.Errorf("处理器不支持Handle方法")
}
