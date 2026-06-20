// Package temu 提供TEMU平台的管道构建器
package temu

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"
)

// PipelineBuilder TEMU管道构建器
type PipelineBuilder struct {
	runtime  pipelineRuntime
	registry *PipelineRegistry
}

// NewPipelineBuilder 创建TEMU管道构建器
func NewPipelineBuilder(runtime pipelineRuntime) *PipelineBuilder {
	return &PipelineBuilder{
		runtime:  runtime,
		registry: NewPipelineRegistry(runtime),
	}
}

// BuildPipeline 构建完整的TEMU管道（使用注册表模式）
func (pb *PipelineBuilder) BuildPipeline() *TemuPipelineExecutor {
	p := pipeline.NewPipeline("TEMU产品发布管道")

	if pb.runtime == nil || pb.runtime.GetStoreClient() == nil || pb.runtime.GetFilterRuleClient() == nil ||
		pb.runtime.GetProductImportMappingClient() == nil || pb.runtime.GetProfitRuleClient() == nil {
		log := logger.GetGlobalLogger("temu.pipeline_builder")
		log.Error("pipeline runtime is not initialized, cannot build TEMU pipeline")
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
		return pipeline.NewProcessError(a.name, "上下文类型错误：期望 *TemuTaskContext", nil)
	}

	return a.temuHandler.HandleTemu(temuCtx)
}
