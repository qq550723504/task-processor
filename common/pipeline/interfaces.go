// Package pipeline 提供通用的任务处理管道接口定义
package pipeline

import (
	"context"
	"task-processor/common/types"
)

// TaskContextInterface 任务上下文通用接口
type TaskContextInterface interface {
	// 基础方法
	GetContext() context.Context
	GetTask() *types.Task

	// 通用数据存储
	SetData(key string, value interface{})
	GetData(key string) (interface{}, bool)
	GetStringData(key string) (string, bool)
	GetIntData(key string) (int, bool)

	// 生命周期管理
	IsCompleted() bool
	SetCompleted(completed bool)
	GetError() error
	SetError(err error)
}

// HandlerInterface 处理器通用接口
type HandlerInterface interface {
	Name() string
	Handle(ctx TaskContextInterface) error
}

// PipelineInterface 管道接口
type PipelineInterface interface {
	AddHandler(handler HandlerInterface)
	Process(ctx TaskContextInterface) error
	GetName() string
}
