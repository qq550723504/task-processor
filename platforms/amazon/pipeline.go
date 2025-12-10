package amazon

import (
	"github.com/sirupsen/logrus"
)

// Pipeline 任务处理管道
type Pipeline struct {
	handlers []StepHandler
}

// NewPipeline 创建新的处理管道
func NewPipeline() *Pipeline {
	return &Pipeline{
		handlers: make([]StepHandler, 0),
	}
}

// AddHandler 添加处理器到管道
func (p *Pipeline) AddHandler(handler StepHandler) *Pipeline {
	p.handlers = append(p.handlers, handler)
	return p
}

// Process 执行管道处理
func (p *Pipeline) Process(ctx *TaskContext) error {
	logrus.Infof("[Amazon] 开始执行任务处理管道，共 %d 个步骤", len(p.handlers))

	for i, handler := range p.handlers {
		stepNum := i + 1
		logrus.Infof("[Amazon] 执行步骤 [%d/%d]: %s", stepNum, len(p.handlers), handler.Name())

		if err := handler.Handle(ctx); err != nil {
			logrus.Errorf("[Amazon] 步骤执行失败 [%d/%d] [%s]: %v",
				stepNum, len(p.handlers), handler.Name(), err)
			return err
		}

		logrus.Infof("[Amazon] 步骤完成 [%d/%d]: %s", stepNum, len(p.handlers), handler.Name())
	}

	logrus.Info("[Amazon] 任务处理管道执行完成")
	return nil
}

// StepHandler 步骤处理器接口
type StepHandler interface {
	Name() string
	Handle(ctx *TaskContext) error
}
