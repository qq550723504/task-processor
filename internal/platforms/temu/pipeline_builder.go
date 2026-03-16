// Package temu 提供TEMU平台的管道构建器
package temu

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
)

// PipelineBuilder TEMU管道构建器
type PipelineBuilder struct {
	processor *TemuProcessor
	registry  *PipelineRegistry
}

// NewPipelineBuilder 创建TEMU管道构建器
func NewPipelineBuilder(processor *TemuProcessor) *PipelineBuilder {
	return &PipelineBuilder{
		processor: processor,
		registry:  NewPipelineRegistry(processor),
	}
}

// BuildPipeline 构建完整的TEMU管道（使用注册表模式）
func (pb *PipelineBuilder) BuildPipeline() *TemuPipelineExecutor {
	p := pipeline.NewPipeline("TEMU产品发布管道")

	// 获取管理客户端
	managementClient := pb.processor.GetManagementClient()
	if managementClient == nil {
		log := logger.GetGlobalLogger("temu.pipeline_builder")
		log.Error("管理客户端未初始化，无法构建管道")
		return NewTemuPipelineExecutor(p)
	}

	// 注册所有处理器
	pb.registry.RegisterAll()

	// 批量添加处理器
	for _, handler := range pb.registry.GetHandlers() {
		p.AddHandler(handler)
	}

	// 返回TEMU专用执行器
	return NewTemuPipelineExecutor(p)
}

// =============================================================================
// 通用强类型适配器
// =============================================================================

// TemuHandler 定义TEMU处理器接口
type TemuHandler interface {
	Name() string
	HandleTemu(*temucontext.TemuTaskContext) error
}

// NewTemuHandlerAdapter 创建通用的TEMU处理器适配器
func NewTemuHandlerAdapter(name string, temuHandler TemuHandler) pipeline.Handler {
	return &temuHandlerAdapter{
		name:        name,
		temuHandler: temuHandler,
	}
}

// temuHandlerAdapter 通用的TEMU处理器适配器
type temuHandlerAdapter struct {
	name        string
	temuHandler TemuHandler
}

// Name 返回处理器名称
func (a *temuHandlerAdapter) Name() string {
	return a.name
}

// Handle 实现pipeline.Handler接口
func (a *temuHandlerAdapter) Handle(ctx pipeline.TaskContext) error {
	// 类型断言转换为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return pipeline.NewHandlerError(a.name, "上下文类型错误：期望 *TemuTaskContext")
	}

	return a.temuHandler.HandleTemu(temuCtx)
}
