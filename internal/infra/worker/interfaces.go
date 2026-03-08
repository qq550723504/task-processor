// Package worker 提供工作池接口定义
package worker

import (
	"context"
)

// Processor 任务处理器接口
// 定义了任务处理器的基本行为
// 注意：Processor 负责解析 WorkerJob 并执行具体的业务逻辑
type Processor interface {
	Start(ctx context.Context) error
	ProcessTask(ctx context.Context, job WorkerJob) error
	Close(ctx context.Context)
}

// JobHandler 任务处理钩子接口
// 用于在任务处理的各个阶段接收通知和处理事件
type JobHandler interface {
	// OnJobStart 任务开始处理时调用
	OnJobStart(job WorkerJob)
	// OnJobSuccess 任务处理成功时调用
	OnJobSuccess(job WorkerJob)
	// OnJobFailure 任务处理失败时调用
	OnJobFailure(job WorkerJob, err error)
	// OnJobPanic 任务处理发生panic时调用
	OnJobPanic(job WorkerJob, panicValue any, stackTrace string)
	// OnJobCompleted 任务处理完成时调用（无论成功或失败）
	OnJobCompleted(job WorkerJob)
}

// WorkerPool 工作池接口
// 提供并发任务调度和管理能力
type WorkerPool interface {
	Start(ctx context.Context)
	Stop(ctx context.Context)
	Submit(job WorkerJob) error
	AvailableSlots() int
	GetQueueStats() QueueStats
	SetJobHandler(handler JobHandler)
}
