// Package alibaba1688 提供任务钩子处理器
package alibaba1688

import (
	"encoding/json"
	"fmt"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

// Crawler1688JobHandler 实现 worker.JobHandler 接口
type Crawler1688JobHandler struct {
	service *Service
}

// parseCrawlTask 从 WorkerJob 中解析 CrawlerTask
func (h *Crawler1688JobHandler) parseCrawlTask(job worker.WorkerJob) (*shared.CrawlerTask, error) {
	var crawlerTask shared.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return nil, err
	}
	return &crawlerTask, nil
}

// OnJobStart 任务开始处理
func (h *Crawler1688JobHandler) OnJobStart(job worker.WorkerJob) {
	crawlerTask, err := h.parseCrawlTask(job)
	if err != nil {
		h.service.logger.Errorf("解析任务失败: %v", err)
		return
	}

	h.service.logger.Infof("🕷️ 开始处理1688任务: %s (URL: %s)", crawlerTask.TaskID, crawlerTask.URL)

	// 更新任务状态为处理中
	h.service.updateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.MarkProcessing()
	})
}

// OnJobSuccess 任务处理成功
func (h *Crawler1688JobHandler) OnJobSuccess(job worker.WorkerJob) {
	crawlerTask, err := h.parseCrawlTask(job)
	if err != nil {
		return
	}

	h.service.logger.Infof("✅ 1688任务成功: %s", crawlerTask.TaskID)

	// 更新结果
	h.service.updateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		// ProductData 已经在 ProcessTask 中设置
		// 这里只需要标记状态
		if result.ProductData != nil {
			result.MarkSuccess(result.ProductData)
		}
	})
}

// OnJobFailure 任务处理失败
func (h *Crawler1688JobHandler) OnJobFailure(job worker.WorkerJob, err error) {
	crawlerTask, parseErr := h.parseCrawlTask(job)
	if parseErr != nil {
		return
	}

	h.service.logger.Errorf("❌ 1688任务失败: %s, 错误: %v", crawlerTask.TaskID, err)

	// 更新结果
	h.service.updateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.MarkFailed(err)
	})
}

// OnJobPanic 任务处理发生panic
func (h *Crawler1688JobHandler) OnJobPanic(job worker.WorkerJob, panicValue any, stackTrace string) {
	crawlerTask, err := h.parseCrawlTask(job)
	if err != nil {
		return
	}

	h.service.logger.Errorf("💥 1688任务Panic: %s, 错误: %v", crawlerTask.TaskID, panicValue)

	// 更新结果
	h.service.updateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.MarkFailed(fmt.Errorf("panic: %v", panicValue))
	})
}

// OnJobCompleted 任务处理完成
func (h *Crawler1688JobHandler) OnJobCompleted(job worker.WorkerJob) {
	// 可以在这里做一些清理工作
}

