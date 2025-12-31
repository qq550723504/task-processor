// Package worker 提供统一的任务处理器基类
package worker

import (
	"context"
	"task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
	"time"

	"github.com/sirupsen/logrus"
)

// BaseTaskHandler 统一的任务处理器基类
// 提供所有平台TaskHandler的通用实现
type BaseTaskHandler struct {
	processor Processor
	logger    *logrus.Entry
}

// NewBaseTaskHandler 创建统一的任务处理器
func NewBaseTaskHandler(processor Processor, platform string) *BaseTaskHandler {
	return &BaseTaskHandler{
		processor: processor,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TaskHandler",
			"platform":  platform,
		}),
	}
}

// ProcessTask 统一的任务处理方法
func (h *BaseTaskHandler) ProcessTask(ctx context.Context, task model.Task, pipeline pipeline.Pipeline) error {
	h.logger.Infof("开始处理任务: ID=%d, ProductID=%s", task.ID, task.ProductID)

	// 记录开始时间
	startTime := time.Now()

	// 委托给具体的处理器执行
	if err := h.processor.ProcessTask(ctx, &task); err != nil {
		h.logger.Errorf("任务处理失败: %v", err)
		return err
	}

	// 记录处理时间
	processTime := time.Since(startTime)
	h.logger.Infof("任务处理成功: ID=%d, 耗时=%v", task.ID, processTime)

	return nil
}

// GetProcessor 获取底层处理器
func (h *BaseTaskHandler) GetProcessor() Processor {
	return h.processor
}
