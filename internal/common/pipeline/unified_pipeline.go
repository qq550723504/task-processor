// Package pipeline 提供统一的管道实现
package pipeline

import (
	"github.com/sirupsen/logrus"
)

// UnifiedPipeline 统一任务处理管道
type UnifiedPipeline struct {
	name     string
	handlers []UnifiedStepHandler
}

// NewUnifiedPipeline 创建新的统一处理管道
func NewUnifiedPipeline(name string) *UnifiedPipeline {
	return &UnifiedPipeline{
		name:     name,
		handlers: make([]UnifiedStepHandler, 0),
	}
}

// AddHandler 添加处理器到管道
func (up *UnifiedPipeline) AddHandler(handler UnifiedStepHandler) *UnifiedPipeline {
	up.handlers = append(up.handlers, handler)
	return up
}

// GetHandlerCount 获取处理器数量
func (up *UnifiedPipeline) GetHandlerCount() int {
	return len(up.handlers)
}

// GetName 获取管道名称
func (up *UnifiedPipeline) GetName() string {
	return up.name
}

// Process 执行管道处理
func (up *UnifiedPipeline) Process(ctx UnifiedTaskContextInterface) error {
	logrus.Infof("[%s] 开始执行统一任务处理管道，共 %d 个步骤", up.name, len(up.handlers))

	for i, handler := range up.handlers {
		stepNum := i + 1
		logrus.Infof("[%s] 执行步骤 [%d/%d]: %s", up.name, stepNum, len(up.handlers), handler.Name())

		if err := handler.Handle(ctx); err != nil {
			logrus.Errorf("[%s] 步骤执行失败 [%d/%d] [%s]: %v",
				up.name, stepNum, len(up.handlers), handler.Name(), err)
			return err
		}

		logrus.Infof("[%s] 步骤完成 [%d/%d]: %s", up.name, stepNum, len(up.handlers), handler.Name())
	}

	logrus.Infof("[%s] 统一任务处理管道执行完成", up.name)
	return nil
}

// GetHandlers 获取所有处理器（只读）
func (up *UnifiedPipeline) GetHandlers() []UnifiedStepHandler {
	// 返回副本以防止外部修改
	handlers := make([]UnifiedStepHandler, len(up.handlers))
	copy(handlers, up.handlers)
	return handlers
}
