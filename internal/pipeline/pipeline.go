// Package pipeline 提供统一的管道实现
package pipeline

import (
	"errors"
	"fmt"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// AbortError 由 handler 返回，通知 pipeline 直接透传原始错误而不做包装。
// 用法：return &pipeline.AbortError{Cause: originalErr}
type AbortError struct {
	Cause error
}

func (e *AbortError) Error() string { return e.Cause.Error() }
func (e *AbortError) Unwrap() error { return e.Cause }

// BasePipeline 统一任务处理管道实现
type BasePipeline struct {
	name     string
	handlers []Handler
	logger   *logrus.Entry
}

// NewPipeline 创建新的处理管道
func NewPipeline(name string) Pipeline {
	return &BasePipeline{
		name:     name,
		handlers: make([]Handler, 0),
		logger: logger.GetGlobalLogger("Pipeline").WithField("pipeline", name),
	}
}

// AddHandler 添加处理器到管道
func (p *BasePipeline) AddHandler(handler Handler) Pipeline {
	p.handlers = append(p.handlers, handler)
	return p
}

// Process 执行管道处理
func (p *BasePipeline) Process(ctx TaskContext) error {
	p.logger.Infof("开始执行管道处理，共 %d 个处理器", len(p.handlers))

	for i, handler := range p.handlers {
		select {
		case <-ctx.GetContext().Done():
			return fmt.Errorf("管道处理被取消: %w", ctx.GetContext().Err())
		default:
		}

		p.logger.Infof("执行处理器 [%d/%d]: %s", i+1, len(p.handlers), handler.Name())

		if err := handler.Handle(ctx); err != nil {
			p.logger.Errorf("处理器 %s 执行失败: %v", handler.Name(), err)
			ctx.SetError(err)

			// handler 通过 AbortError 标记错误不需要被包装，直接透传
			var abortErr *AbortError
			if errors.As(err, &abortErr) {
				return abortErr.Cause
			}

			return fmt.Errorf("处理器 %s 执行失败: %w", handler.Name(), err)
		}

		p.logger.Infof("处理器 %s 执行成功", handler.Name())
	}

	p.logger.Info("管道处理完成")
	ctx.SetCompleted(true)
	return nil
}

// GetHandlerCount 获取处理器数量
func (p *BasePipeline) GetHandlerCount() int {
	return len(p.handlers)
}

// GetName 获取管道名称
func (p *BasePipeline) GetName() string {
	return p.name
}

// GetHandlers 获取所有处理器（只读副本）
func (p *BasePipeline) GetHandlers() []Handler {
	handlers := make([]Handler, len(p.handlers))
	copy(handlers, p.handlers)
	return handlers
}

var _ Pipeline = (*BasePipeline)(nil)
