// Package distributed 提供分布式爬虫类型定义
package distributed

import (
	"context"
	"time"

	"task-processor/internal/model"
)

// CrawlRequest 爬虫请求
type CrawlRequest struct {
	TaskID         string            `json:"taskId"` // 字符串类型，避免 JSON float64 精度丢失
	TenantID       int64             `json:"tenantId"`
	StoreID        int64             `json:"storeId"`
	Platform       string            `json:"platform"` // Queue naming only; not sent as routing data.
	SourcePlatform string            `json:"sourcePlatform"`
	TargetPlatform string            `json:"targetPlatform"`
	Region         string            `json:"region"`
	ProductID      string            `json:"productId"`
	Zipcode        string            `json:"zipcode,omitempty"`
	Priority       int               `json:"priority"`
	TraceID        string            `json:"traceId,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// CrawlResult 爬虫结果
type CrawlResult struct {
	TaskID   string         `json:"taskId"` // 字符串类型，避免 JSON float64 精度丢失
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
