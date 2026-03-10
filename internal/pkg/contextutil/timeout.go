// Package contextutil 提供统一的上下文超时管理工具
package contextutil

import (
	"context"
	"time"
)

// 预定义的超时常量
const (
	// AI相关超时
	AITimeout      = 60 * time.Second // AI调用标准超时
	AIShortTimeout = 30 * time.Second // AI快速调用超时
	AILongTimeout  = 2 * time.Minute  // AI长时间调用超时

	// HTTP相关超时
	HTTPTimeout      = 10 * time.Second // HTTP请求标准超时
	HTTPShortTimeout = 5 * time.Second  // HTTP快速请求超时
	HTTPLongTimeout  = 30 * time.Second // HTTP长时间请求超时

	// 任务处理超时
	TaskTimeout      = 2 * time.Minute  // 任务处理标准超时
	TaskShortTimeout = 30 * time.Second // 任务快速处理超时
	TaskLongTimeout  = 5 * time.Minute  // 任务长时间处理超时
	TaskExtraTimeout = 60 * time.Minute // 任务超长处理超时

	// 下载相关超时
	DownloadTimeout     = 10 * time.Minute // 下载标准超时
	DownloadLongTimeout = 30 * time.Minute // 下载长时间超时

	// 系统操作超时
	ShutdownTimeout = 30 * time.Second // 系统关闭超时
	HealthTimeout   = 10 * time.Second // 健康检查超时
)

// WithAITimeout 创建AI调用超时上下文 (60秒)
func WithAITimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, AITimeout)
}

// WithAIShortTimeout 创建AI快速调用超时上下文 (30秒)
func WithAIShortTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, AIShortTimeout)
}

// WithAILongTimeout 创建AI长时间调用超时上下文 (2分钟)
func WithAILongTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, AILongTimeout)
}

// WithHTTPTimeout 创建HTTP请求超时上下文 (10秒)
func WithHTTPTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, HTTPTimeout)
}

// WithHTTPShortTimeout 创建HTTP快速请求超时上下文 (5秒)
func WithHTTPShortTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, HTTPShortTimeout)
}

// WithHTTPLongTimeout 创建HTTP长时间请求超时上下文 (30秒)
func WithHTTPLongTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, HTTPLongTimeout)
}

// WithTaskTimeout 创建任务处理超时上下文 (2分钟)
func WithTaskTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, TaskTimeout)
}

// WithTaskShortTimeout 创建任务快速处理超时上下文 (30秒)
func WithTaskShortTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, TaskShortTimeout)
}

// WithTaskLongTimeout 创建任务长时间处理超时上下文 (5分钟)
func WithTaskLongTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, TaskLongTimeout)
}

// WithTaskExtraTimeout 创建任务超长处理超时上下文 (60分钟)
func WithTaskExtraTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, TaskExtraTimeout)
}

// WithDownloadTimeout 创建下载超时上下文 (10分钟)
func WithDownloadTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, DownloadTimeout)
}

// WithShutdownTimeout 创建系统关闭超时上下文 (30秒)
func WithShutdownTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, ShutdownTimeout)
}

// WithHealthTimeout 创建健康检查超时上下文 (10秒)
func WithHealthTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, HealthTimeout)
}

// WithCustomTimeout 创建自定义超时上下文
func WithCustomTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}
