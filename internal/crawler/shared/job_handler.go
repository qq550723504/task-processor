// Package shared 提供爬虫共享任务处理器基础实现
package shared

import (
	"encoding/json"
	"fmt"

	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// BaseJobHandler 通用任务钩子处理器，amazon 和 alibaba1688 均可复用。
// Name 用于日志前缀区分来源（如 "Amazon"、"1688"）。
type BaseJobHandler struct {
	Name         string
	Logger       *logrus.Logger
	UpdateResult func(taskID string, fn func(*CrawlerResult))
}

// parseCrawlTask 从 WorkerJob 中解析 CrawlerTask
func (h *BaseJobHandler) parseCrawlTask(job worker.WorkerJob) (*CrawlerTask, error) {
	var task CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// OnJobStart 任务开始处理
func (h *BaseJobHandler) OnJobStart(job worker.WorkerJob) {
	task, err := h.parseCrawlTask(job)
	if err != nil {
		h.Logger.Errorf("解析任务失败: %v", err)
		return
	}
	h.Logger.Infof("🕷️ 开始处理%s任务: %s (URL: %s)", h.Name, task.TaskID, task.URL)
	h.UpdateResult(task.TaskID, func(r *CrawlerResult) { r.MarkProcessing() })
}

// OnJobSuccess 任务处理成功
func (h *BaseJobHandler) OnJobSuccess(job worker.WorkerJob) {
	task, err := h.parseCrawlTask(job)
	if err != nil {
		return
	}
	h.Logger.Infof("✅ %s任务成功: %s", h.Name, task.TaskID)
	h.UpdateResult(task.TaskID, func(r *CrawlerResult) {
		if r.ProductData != nil {
			r.MarkSuccess(r.ProductData)
		}
	})
}

// OnJobFailure 任务处理失败
func (h *BaseJobHandler) OnJobFailure(job worker.WorkerJob, err error) {
	task, parseErr := h.parseCrawlTask(job)
	if parseErr != nil {
		return
	}
	h.Logger.Errorf("❌ %s任务失败: %s, 错误: %v", h.Name, task.TaskID, err)
	h.UpdateResult(task.TaskID, func(r *CrawlerResult) { r.MarkFailed(err) })
}

// OnJobPanic 任务处理发生 panic
func (h *BaseJobHandler) OnJobPanic(job worker.WorkerJob, panicValue any, _ string) {
	task, err := h.parseCrawlTask(job)
	if err != nil {
		return
	}
	h.Logger.Errorf("💥 %s任务Panic: %s, 错误: %v", h.Name, task.TaskID, panicValue)
	h.UpdateResult(task.TaskID, func(r *CrawlerResult) {
		r.MarkFailed(fmt.Errorf("panic: %v", panicValue))
	})
}

// OnJobCompleted 任务处理完成（留空，子类可按需覆盖）
func (h *BaseJobHandler) OnJobCompleted(_ worker.WorkerJob) {}
