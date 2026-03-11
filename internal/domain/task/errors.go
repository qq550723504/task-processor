// Package task 提供任务相关错误定义
package task

import "errors"

var (
	// ErrInvalidTask 无效的任务
	ErrInvalidTask = errors.New("无效的任务：URL 或 ASIN 必须提供一个")

	// ErrTaskNotFound 任务不存在
	ErrTaskNotFound = errors.New("任务不存在")

	// ErrQueueFull 任务队列已满
	ErrQueueFull = errors.New("任务队列已满")
)
