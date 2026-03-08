// Package worker 提供工作协程实现
package worker

import (
	"context"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

// Worker 工作协程
type Worker struct {
	id        int
	pool      *Pool
	jobQueue  <-chan WorkerJob
	processor Processor
	logger    *logrus.Entry
}

// Run 工作协程运行
// 从任务队列中获取任务并处理，直到上下文取消或队列关闭
func (w *Worker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	w.logger.WithField("worker_id", w.id).Debug("工作协程已启动")

	for {
		select {
		case <-ctx.Done():
			w.logger.WithField("worker_id", w.id).Debug("工作协程正在停止")
			return
		case job, ok := <-w.jobQueue:
			if !ok {
				w.logger.WithField("worker_id", w.id).Debug("工作协程任务队列已关闭")
				return
			}

			w.processJob(ctx, job)
		}
	}
}

// processJob 处理单个任务
func (w *Worker) processJob(ctx context.Context, job WorkerJob) {
	// 使用 defer 和 recover 确保 panic 不会导致工作协程崩溃
	defer func() {
		if r := recover(); r != nil {
			w.handlePanic(r, job)
		}
	}()

	// 获取钩子处理器（线程安全）
	jobHandler := w.pool.getJobHandler()

	// 记录任务开始处理
	if w.pool.metrics != nil {
		w.pool.metrics.RecordProcessStart(job.TaskID)
	}

	// 通知任务开始
	if jobHandler != nil {
		jobHandler.OnJobStart(job)
	}

	// 确保任务处理完成后通知（无论成功或失败）
	defer func() {
		if jobHandler != nil {
			jobHandler.OnJobCompleted(job)
		}
	}()

	// 设置任务处理超时时间
	processCtx, cancel := context.WithTimeout(ctx, w.pool.config.TaskTimeout)
	defer cancel()

	// 执行任务处理
	if err := w.processor.ProcessTask(processCtx, job); err != nil {
		w.handleTaskFailure(job, err)
	} else {
		w.handleTaskSuccess(job)
	}
}

// handleTaskSuccess 处理任务成功
func (w *Worker) handleTaskSuccess(job WorkerJob) {
	// 记录成功指标
	if w.pool.metrics != nil {
		w.pool.metrics.RecordProcessSuccess(job.TaskID)
	}

	// 通知钩子
	jobHandler := w.pool.getJobHandler()
	if jobHandler != nil {
		jobHandler.OnJobSuccess(job)
	}
}

// handleTaskFailure 处理任务失败
func (w *Worker) handleTaskFailure(job WorkerJob, err error) {
	// 记录失败指标
	if w.pool.metrics != nil {
		w.pool.metrics.RecordProcessFailure(job.TaskID)
	}

	// 通知钩子
	jobHandler := w.pool.getJobHandler()
	if jobHandler != nil {
		jobHandler.OnJobFailure(job, err)
	}
}

// handlePanic 处理Panic
func (w *Worker) handlePanic(r any, job WorkerJob) {
	// 获取堆栈跟踪
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	// 记录panic指标
	if w.pool.metrics != nil {
		w.pool.metrics.RecordPanic(job.TaskID)
	}

	// 记录到日志（基础设施层的基本日志）
	w.logger.WithFields(map[string]any{
		"worker_id":   w.id,
		"task_id":     job.TaskID,
		"panic":       r,
		"stack_trace": stackTrace,
	}).Error("工作协程发生panic")

	// 通知业务层处理
	jobHandler := w.pool.getJobHandler()
	if jobHandler != nil {
		jobHandler.OnJobPanic(job, r, stackTrace)
	}
}
