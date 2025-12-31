// Package worker 提供工作池相关数据结构
package worker

// QueueStats 队列统计信息
type QueueStats struct {
	QueueSize      int     // 当前队列中的任务数
	BufferSize     int     // 队列总容量
	AvailableSlots int     // 可用槽位数
	UsagePercent   float64 // 使用率（%）
}

// WorkerJob 工作任务
type WorkerJob struct {
	TenantID string
	ShopID   string
	TaskData string
}
