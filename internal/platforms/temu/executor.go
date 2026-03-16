// Package temu 提供TEMU平台专用的管道执行器
package temu

import (
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
)

// TemuPipelineExecutor TEMU专用管道执行器
type TemuPipelineExecutor struct {
	pipeline pipeline.Pipeline
}

// NewTemuPipelineExecutor 创建TEMU管道执行器
func NewTemuPipelineExecutor(p pipeline.Pipeline) *TemuPipelineExecutor {
	return &TemuPipelineExecutor{
		pipeline: p,
	}
}

// Execute 执行管道
func (e *TemuPipelineExecutor) Execute(ctx *temucontext.TemuTaskContext) error {
	// 直接使用pipeline的Process方法，TemuTaskContext实现了TaskContext接口
	return e.pipeline.Process(ctx)
}

// GetPipeline 获取底层管道
func (e *TemuPipelineExecutor) GetPipeline() pipeline.Pipeline {
	return e.pipeline
}
