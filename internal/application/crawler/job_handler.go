// Package crawler 提供任务钩子处理器
package crawler

import (
	"encoding/json"
	"fmt"

	"task-processor/internal/domain/task"
	"task-processor/internal/infra/worker"
)

// CrawlerJobHandler 实现 worker.JobHandler 接口
type CrawlerJobHandler struct {
	service *Service
}

// parseCrawlTask 从 WorkerJob 中解析 CrawlerTask
func (h *CrawlerJobHandler) parseCrawlTask(job worker.WorkerJob) (*task.CrawlerTask, error) {
	var crawlerTask task.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return nil, err
	}
	return &crawlerTask, nil
}

// OnJobStart 任务开始处理
func (h *CrawlerJobHandler) OnJobStart(job worker.WorkerJob) {
	crawlerTask, err := h.parseCrawlTask(job)
	if err != nil {
		h.service.logger.Errorf("解析任务失败: %v", err)
		return
	}

	h.service.logger.Infof("🕷️ 开始处理任务: %s (URL: %s)", crawlerTask.TaskID, crawlerTask.URL)

	// 更新任务状态为处理中
	h.service.updateResult(crawlerTask.TaskID, func(result *task.CrawlerResult) {
		result.MarkProcessing()
	})
}

// OnJobSuccess 任务处理成功
func (h *CrawlerJobHandler) OnJobSuccess(job worker.WorkerJob) {
	crawlerTask, err := h.parseCrawlTask(job)
	if err != nil {
		return
	}

	h.service.logger.Infof("✅ 任务成功: %s", crawlerTask.TaskID)

	// 更新结果
	h.service.updateResult(crawlerTask.TaskID, func(result *task.CrawlerResult) {
		// ProductData 已经在 ProcessTask 中设置
		// 这里只需要标记状态
		if result.ProductData != nil {
			result.MarkSuccess(result.ProductData)
		}
	})
}

// OnJobFailure 任务处理失败
func (h *CrawlerJobHandler) OnJobFailure(job worker.WorkerJob, err error) {
	crawlerTask, parseErr := h.parseCrawlTask(job)
	if parseErr != nil {
		return
	}

	h.service.logger.Errorf("❌ 任务失败: %s, 错误: %v", crawlerTask.TaskID, err)

	// 更新结果
	h.service.updateResult(crawlerTask.TaskID, func(result *task.CrawlerResult) {
		result.MarkFailed(err)
	})
}

// OnJobPanic 任务处理发生panic
func (h *CrawlerJobHandler) OnJobPanic(job worker.WorkerJob, panicValue any, stackTrace string) {
	crawlerTask, err := h.parseCrawlTask(job)
	if err != nil {
		return
	}

	h.service.logger.Errorf("💥 任务Panic: %s, 错误: %v", crawlerTask.TaskID, panicValue)

	// 更新结果
	h.service.updateResult(crawlerTask.TaskID, func(result *task.CrawlerResult) {
		result.MarkFailed(fmt.Errorf("panic: %v", panicValue))
	})
}

// OnJobCompleted 任务处理完成
func (h *CrawlerJobHandler) OnJobCompleted(job worker.WorkerJob) {
	// 可以在这里做一些清理工作
}
