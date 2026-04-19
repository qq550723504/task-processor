// Package shared 提供爬虫共享错误定义
package shared

import "errors"

var (
	// ErrInvalidCrawlerTask 无效的爬虫任务
	ErrInvalidCrawlerTask = errors.New("无效的爬虫任务：URL 或 ASIN 必须提供一个")

	// ErrTaskNotFound 任务不存在
	ErrTaskNotFound = errors.New("任务不存在")

	// ErrSharedResultStoreUnavailable 共享任务结果存储不可用
	ErrSharedResultStoreUnavailable = errors.New("异步任务共享结果存储不可用")

	// ErrQueueFull 任务队列已满
	ErrQueueFull = errors.New("任务队列已满")
)
