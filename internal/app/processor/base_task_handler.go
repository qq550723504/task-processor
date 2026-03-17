// Package processor 提供统一的任务处理器基类
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pipeline"
	"time"

	"github.com/sirupsen/logrus"
)

// BaseTaskHandler 统一的任务处理器基类
// 提供所有平台TaskHandler的通用实现
type BaseTaskHandler struct {
	processor worker.Processor
	logger    *logrus.Entry
}

// NewBaseTaskHandler 创建统一的任务处理器
func NewBaseTaskHandler(processor worker.Processor, platform string) *BaseTaskHandler {
	log := logger.GetGlobalLogger("worker.task_handler").WithFields(map[string]any{
		logger.FieldComponent: "TaskHandler",
		logger.FieldPlatform:  platform,
	})
	return &BaseTaskHandler{
		processor: processor,
		logger:    log,
	}
}

// ProcessTask 统一的任务处理方法
func (h *BaseTaskHandler) ProcessTask(ctx context.Context, task model.Task, pipeline pipeline.Pipeline) error {
	// 将任务转换为 WorkerJob
	taskData, err := json.Marshal(task)
	if err != nil {
		h.logger.WithError(err).Error("序列化任务数据失败")
		return err
	}

	job := worker.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: string(taskData),
	}

	h.logger.WithFields(map[string]any{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
	}).Info("开始处理任务")

	// 记录开始时间
	startTime := time.Now()

	// 委托给具体的处理器执行
	if err := h.processor.ProcessTask(ctx, job); err != nil {
		h.logger.WithError(err).Error("任务处理失败")
		return err
	}

	// 记录处理时间
	processTime := time.Since(startTime)
	h.logger.WithFields(map[string]any{
		logger.FieldTaskID:     task.ID,
		logger.FieldDurationMs: processTime.Milliseconds(),
	}).Info("任务处理成功")

	return nil
}

// GetProcessor 获取底层处理器
func (h *BaseTaskHandler) GetProcessor() worker.Processor {
	return h.processor
}

