// Package handler 提供Amazon平台处理器接口定义
package handler

import (
	"task-processor/platforms/amazon/internal/model"
)

// Handler Amazon处理器接口
// 使用依赖注入模式，避免循环导入
type Handler interface {
	Name() string
	Execute(services *model.Services, data map[string]any) error
}

// HandlerFunc 处理器函数类型
type HandlerFunc func(services *model.Services, data map[string]any) error

// Name 返回函数处理器的名称
func (hf HandlerFunc) Name() string {
	return "HandlerFunc"
}

// Execute 执行处理器函数
func (hf HandlerFunc) Execute(services *model.Services, data map[string]any) error {
	return hf(services, data)
}
