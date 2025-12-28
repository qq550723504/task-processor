// Package pipeline 提供统一的任务处理管道接口定义
package pipeline

// Handler 处理器接口
type Handler interface {
	Name() string
	Handle(ctx TaskContext) error
}

// Pipeline 管道接口
type Pipeline interface {
	AddHandler(handler Handler) Pipeline
	Process(ctx TaskContext) error
	GetName() string
	GetHandlerCount() int
}
