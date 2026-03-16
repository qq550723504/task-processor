// Package service 定义领域服务接口
package service

import (
	"task-processor/internal/domain/task"
)

// HealthChecker 健康检查接口
type HealthChecker interface {
	IsReady() bool
	IsHealthy() bool
}

// CrawlerService 爬虫服务接口
// 定义爬虫服务的核心能力,供基础设施层(如HTTP handler)使用
type CrawlerService interface {
	HealthChecker

	// SubmitTask 提交爬虫任务
	SubmitTask(crawlerTask *task.CrawlerTask) error

	// GetTask 获取任务结果
	GetTask(taskID string) (*task.CrawlerResult, error)

	// DeleteTask 删除任务
	DeleteTask(taskID string)

	// GetAllTasks 获取所有任务
	GetAllTasks() []*task.CrawlerResult

	// GetStats 获取统计信息
	GetStats() map[string]interface{}
}
