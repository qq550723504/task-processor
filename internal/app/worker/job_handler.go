// Package worker 提供应用层的任务处理钩子实现
package worker

import (
	"encoding/json"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// TaskJobHandler 任务处理钩子实现
// 负责处理任务生命周期事件，包括日志记录、指标收集、任务完成通知等
type TaskJobHandler struct {
	logger             *logrus.Logger
	metrics            *worker.Metrics
	completionNotifier TaskCompletionNotifier
}

// TaskCompletionNotifier 任务完成通知接口
// 用于在任务完成时接收通知
type TaskCompletionNotifier interface {
	OnTaskCompleted(taskID int64)
}

// NewTaskJobHandler 创建任务处理钩子
func NewTaskJobHandler(
	logger *logrus.Logger,
	metrics *worker.Metrics,
	completionNotifier TaskCompletionNotifier,
) *TaskJobHandler {
	return &TaskJobHandler{
		logger:             logger,
		metrics:            metrics,
		completionNotifier: completionNotifier,
	}
}

// OnJobStart 任务开始处理时调用
func (h *TaskJobHandler) OnJobStart(job worker.WorkerJob) {
	task := h.parseTask(job)
	if task == nil {
		return
	}

	// 记录开始处理
	if h.metrics != nil {
		h.metrics.RecordProcessStart(task.ID)
	}

	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
		"tenant_id":           job.TenantID,
		"shop_id":             job.ShopID,
	}).Info("开始处理任务")
}

// OnJobSuccess 任务处理成功时调用
func (h *TaskJobHandler) OnJobSuccess(job worker.WorkerJob) {
	task := h.parseTask(job)
	if task == nil {
		return
	}

	// 记录成功
	if h.metrics != nil {
		h.metrics.RecordProcessSuccess(task.ID)
	}

	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
	}).Info("任务处理完成")
}

// OnJobFailure 任务处理失败时调用
func (h *TaskJobHandler) OnJobFailure(job worker.WorkerJob, err error) {
	task := h.parseTask(job)
	if task == nil {
		return
	}

	// 记录失败
	if h.metrics != nil {
		h.metrics.RecordProcessFailure(task.ID)
	}

	h.logger.WithFields(logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
	}).WithError(err).Error("处理任务失败")
}

// OnJobPanic 任务处理发生panic时调用
func (h *TaskJobHandler) OnJobPanic(job worker.WorkerJob, panicValue any, stackTrace string) {
	task := h.parseTask(job)

	// 记录Panic
	if h.metrics != nil && task != nil {
		h.metrics.RecordPanic(task.ID)
	}

	fields := logrus.Fields{
		"panic":       panicValue,
		"stack_trace": stackTrace,
		"tenant_id":   job.TenantID,
		"shop_id":     job.ShopID,
	}

	if task != nil {
		fields[logger.FieldTaskID] = task.ID
		fields[logger.FieldProductID] = task.ProductID
	}

	h.logger.WithFields(fields).Error("任务处理发生panic")
}

// OnJobCompleted 任务处理完成时调用（无论成功或失败）
func (h *TaskJobHandler) OnJobCompleted(job worker.WorkerJob) {
	task := h.parseTask(job)
	if task == nil {
		return
	}

	// 通知任务完成
	if h.completionNotifier != nil {
		h.completionNotifier.OnTaskCompleted(task.ID)
	}
}

// parseTask 解析任务数据
func (h *TaskJobHandler) parseTask(job worker.WorkerJob) *model.Task {
	var task model.Task
	if err := json.Unmarshal([]byte(job.TaskData), &task); err != nil {
		h.logger.WithFields(logrus.Fields{
			"tenant_id": job.TenantID,
			"shop_id":   job.ShopID,
		}).WithError(err).Error("解析任务数据失败")
		return nil
	}
	return &task
}

