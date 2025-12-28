// Package worker 提供工作池接口定义
package worker

import (
	"context"
	"task-processor/internal/types"
)

// Processor 任务处理器接口
type Processor interface {
	Start(ctx context.Context) error
	ProcessTask(ctx context.Context, task *types.Task) error
	Close(ctx context.Context)
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

// TaskCompletionNotifier 任务完成通知接口
type TaskCompletionNotifier interface {
	OnTaskCompleted(taskID int64)
}

// TaskSubmitter 任务提交器接口
type TaskSubmitter interface {
	SubmitTask(ctx context.Context, taskData string) error
	GetQueueStats() QueueStats
}
