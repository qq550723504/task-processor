package processor

import (
	"context"
	"task-processor/internal/common/types"
)

// Processor 任务处理器接口
type Processor interface {
	Start(ctx context.Context) error
	ProcessTask(ctx context.Context, task *types.Task) error
	Close()
}

// TaskCompletionNotifier 任务完成通知接口
type TaskCompletionNotifier interface {
	OnTaskCompleted(taskID string)
}

// WorkerPool 工作池接口
type WorkerPool interface {
	Start(ctx context.Context)
	Stop(ctx context.Context)
	Submit(job WorkerJob) error
	AvailableSlots() int
	GetQueueStats() QueueStats
	SetCompletionNotifier(notifier TaskCompletionNotifier)
}

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
