// Package crawler 提供分布式爬虫客户端
package crawler

import (
	"context"
	"time"

	"task-processor/internal/domain/model"
)

// CrawlRequest 爬虫请求
type CrawlRequest struct {
	TaskID    int64  `json:"taskId"`
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	Platform  string `json:"platform"`
	Region    string `json:"region"`
	ProductID string `json:"productId"`
	Priority  int    `json:"priority"`
}

// CrawlResult 爬虫结果
type CrawlResult struct {
	TaskID   int64          `json:"taskId"`
	Success  bool           `json:"success"`
	Product  *model.Product `json:"product,omitempty"`
	Error    string         `json:"error,omitempty"`
	Duration time.Duration  `json:"duration"`
	NodeID   string         `json:"nodeId"`
}

// PendingTask 等待中的任务
type PendingTask struct {
	TaskID     string
	ResultChan chan *CrawlResult
	CreatedAt  time.Time
	Context    context.Context
	Cancel     context.CancelFunc
}
