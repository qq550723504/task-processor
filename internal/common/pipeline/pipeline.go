package pipeline

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// Handler 处理器接口
type Handler interface {
	// Name 返回处理器名称
	Name() string

	// Handle 执行处理逻辑
	Handle(ctx *TaskContext) error
}

// Pipeline 处理管道
type Pipeline struct {
	name     string
	handlers []Handler
	logger   *logrus.Entry
}

// NewPipeline 创建新的处理管道
func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name:     name,
		handlers: make([]Handler, 0),
		logger: logrus.WithFields(logrus.Fields{
			"component": "Pipeline",
			"pipeline":  name,
		}),
	}
}

// AddHandler 添加处理器到管道
func (p *Pipeline) AddHandler(handler Handler) *Pipeline {
	p.handlers = append(p.handlers, handler)
	return p
}

// Process 执行管道处理
func (p *Pipeline) Process(ctx *TaskContext) error {
	p.logger.Infof("开始执行管道处理，共 %d 个处理器", len(p.handlers))

	for i, handler := range p.handlers {
		// 检查上下文是否已取消
		select {
		case <-ctx.Context.Done():
			return fmt.Errorf("管道处理被取消: %w", ctx.Context.Err())
		default:
		}

		p.logger.Infof("执行处理器 [%d/%d]: %s", i+1, len(p.handlers), handler.Name())

		if err := handler.Handle(ctx); err != nil {
			p.logger.Errorf("处理器 %s 执行失败: %v", handler.Name(), err)
			return fmt.Errorf("处理器 %s 执行失败: %w", handler.Name(), err)
		}

		p.logger.Infof("处理器 %s 执行成功", handler.Name())
	}

	p.logger.Info("管道处理完成")
	return nil
}

// GetHandlerCount 获取处理器数量
func (p *Pipeline) GetHandlerCount() int {
	return len(p.handlers)
}

// GetName 获取管道名称
func (p *Pipeline) GetName() string {
	return p.name
}
