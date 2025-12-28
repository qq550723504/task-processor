// Package pipeline 管道并行处理器
package pipeline

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// ParallelHandler 并行执行多个handler
type ParallelHandler struct {
	name     string
	handlers []Handler
	logger   *logrus.Entry
}

// NewParallelHandler 创建并行handler
func NewParallelHandler(name string, handlers ...Handler) *ParallelHandler {
	return &ParallelHandler{
		name:     name,
		handlers: handlers,
		logger:   logrus.WithField("handler", "ParallelHandler"),
	}
}

// Name 返回处理器名称
func (h *ParallelHandler) Name() string {
	return h.name
}

// Handle 并行执行所有handler
func (h *ParallelHandler) Handle(ctx TaskContext) error {
	if len(h.handlers) == 0 {
		return nil
	}

	// 如果只有一个handler，直接执行
	if len(h.handlers) == 1 {
		return h.handlers[0].Handle(ctx)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(h.handlers))

	h.logger.Infof("🔄 开始并行执行 %d 个handler", len(h.handlers))

	// 并行执行所有handler
	for _, handler := range h.handlers {
		wg.Add(1)
		go func(hd Handler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					h.logger.Errorf("❌ Handler %s 发生panic: %v", hd.Name(), r)
					errChan <- fmt.Errorf("handler %s panic: %v", hd.Name(), r)
				}
			}()

			h.logger.Infof("  ▶️ 开始执行: %s", hd.Name())
			if err := hd.Handle(ctx); err != nil {
				h.logger.Errorf("  ❌ %s 执行失败: %v", hd.Name(), err)
				errChan <- fmt.Errorf("%s: %w", hd.Name(), err)
			} else {
				h.logger.Infof("  ✅ %s 执行完成", hd.Name())
			}
		}(handler)
	}

	// 等待所有handler完成
	wg.Wait()
	close(errChan)

	// 收集错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// 如果有错误，返回第一个错误
	if len(errors) > 0 {
		h.logger.Errorf("❌ 并行执行失败，共 %d 个错误", len(errors))
		return errors[0]
	}

	h.logger.Infof("✅ 所有并行handler执行完成")
	return nil
}
