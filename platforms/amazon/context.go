// Package amazon 提供Amazon平台核心上下文定义
package amazon

import (
	"context"
	"task-processor/common/pipeline"
	"task-processor/common/types"
)

// TaskContext Amazon任务处理上下文
// 组合通用BaseTaskContext，遵循组合优于继承原则
type TaskContext struct {
	*pipeline.BaseTaskContext
}

// NewTaskContext 创建Amazon任务上下文
func NewTaskContext(ctx context.Context, task *types.Task) *TaskContext {
	return &TaskContext{
		BaseTaskContext: pipeline.NewBaseTaskContext(ctx, task),
	}
}

// StepHandler Amazon步骤处理器接口
type StepHandler interface {
	Name() string
	Handle(ctx *TaskContext) error
}
