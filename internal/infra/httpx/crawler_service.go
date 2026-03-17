// Package handler 提供 HTTP 处理器
package httpx

import "task-processor/internal/crawler/shared"

// CrawlerService 爬虫服务接口，由 HTTP handler 消费
type CrawlerService interface {
	HealthChecker

	// SubmitTask 提交爬虫任务
	SubmitTask(crawlerTask *shared.CrawlerTask) error

	// GetTask 获取任务结果
	GetTask(taskID string) (*shared.CrawlerResult, error)

	// DeleteTask 删除任务
	DeleteTask(taskID string)

	// GetAllTasks 获取所有任务
	GetAllTasks() []*shared.CrawlerResult

	// GetStats 获取统计信息
	GetStats() map[string]any
}
