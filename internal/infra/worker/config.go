// Package worker 提供工作池配置
package worker

import (
	"fmt"
	"time"
)

// PoolConfig 工作池配置
type PoolConfig struct {
	// 并发数
	Concurrency int
	// 队列缓冲区大小
	BufferSize int
	// 任务超时时间
	TaskTimeout time.Duration
	// 是否启用指标收集
	EnableMetrics bool
	// 优雅关闭超时
	ShutdownTimeout time.Duration
}

// DefaultPoolConfig 返回默认配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		Concurrency:     5,
		BufferSize:      100,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	}
}

// Validate 验证配置并自动修正无效值
// 返回警告信息,如果配置被修正
func (c *PoolConfig) Validate() error {
	var warnings []string

	if c.Concurrency <= 0 {
		warnings = append(warnings, fmt.Sprintf("并发数无效(%d),已修正为1", c.Concurrency))
		c.Concurrency = 1
	}
	if c.BufferSize <= 0 {
		warnings = append(warnings, fmt.Sprintf("缓冲区大小无效(%d),已修正为10", c.BufferSize))
		c.BufferSize = 10
	}
	if c.TaskTimeout <= 0 {
		warnings = append(warnings, fmt.Sprintf("任务超时时间无效(%v),已修正为15分钟", c.TaskTimeout))
		c.TaskTimeout = 15 * time.Minute
	}
	if c.ShutdownTimeout <= 0 {
		warnings = append(warnings, fmt.Sprintf("关闭超时时间无效(%v),已修正为30秒", c.ShutdownTimeout))
		c.ShutdownTimeout = 30 * time.Second
	}

	if len(warnings) > 0 {
		return fmt.Errorf("配置已自动修正: %v", warnings)
	}
	return nil
}
