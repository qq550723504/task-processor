// package pipeline 提供Amazon平台处理器接口定义
package pipeline

import (
	"context"
	"task-processor/internal/amazon/model"
)

// Handler Amazon处理器接口
// 使用依赖注入模式，避免循环导入
type Handler interface {
	Name() string
	Handle(ctx context.Context, taskContext *model.TaskContext) error
}

// HandlerFunc 处理器函数类型
type HandlerFunc func(ctx context.Context, taskContext *model.TaskContext) error

// Name 返回函数处理器的名称
func (hf HandlerFunc) Name() string {
	return "HandlerFunc"
}

// Handle 执行处理器函数
func (hf HandlerFunc) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	return hf(ctx, taskContext)
}

