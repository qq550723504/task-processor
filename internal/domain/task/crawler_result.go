// Package task 提供任务结果领域模型
package task

import (
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusSuccess    TaskStatus = "success"
	StatusFailed     TaskStatus = "failed"
)

// CrawlerResult 爬虫任务结果
type CrawlerResult struct {
	TaskID      string                 // 任务 ID
	Status      TaskStatus             // 任务状态
	ProductData map[string]any // 产品数据
	Error       string                 // 错误信息
	StartedAt   *time.Time             // 开始时间
	CompletedAt *time.Time             // 完成时间
	Duration    string                 // 执行时长
}

// NewCrawlerResult 创建任务结果
func NewCrawlerResult(taskID string) *CrawlerResult {
	return &CrawlerResult{
		TaskID: taskID,
		Status: StatusPending,
	}
}

// MarkProcessing 标记为处理中
func (r *CrawlerResult) MarkProcessing() {
	now := time.Now()
	r.Status = StatusProcessing
	r.StartedAt = &now
}

// MarkSuccess 标记为成功
func (r *CrawlerResult) MarkSuccess(productData map[string]any) {
	now := time.Now()
	r.Status = StatusSuccess
	r.ProductData = productData
	r.CompletedAt = &now
	if r.StartedAt != nil {
		r.Duration = now.Sub(*r.StartedAt).String()
	}
}

// MarkFailed 标记为失败
func (r *CrawlerResult) MarkFailed(err error) {
	now := time.Now()
	r.Status = StatusFailed
	r.Error = err.Error()
	r.CompletedAt = &now
	if r.StartedAt != nil {
		r.Duration = now.Sub(*r.StartedAt).String()
	}
}

// IsCompleted 是否已完成
func (r *CrawlerResult) IsCompleted() bool {
	return r.Status == StatusSuccess || r.Status == StatusFailed
}
