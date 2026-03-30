// Package shared 提供爬虫共享服务基础实现
package shared

import (
	"sync"

	"task-processor/internal/infra/worker"
)

// BaseService 封装两个爬虫服务共同的结果存储与 worker 池逻辑。
// amazon.Service 和 alibaba1688.Service 通过嵌入此结构体消除重复代码。
type BaseService struct {
	workerPool worker.WorkerPool
	results    sync.Map
	resultMu   sync.Mutex
}

// SetWorkerPool 设置 worker 池（由子类在构造时调用）
func (b *BaseService) SetWorkerPool(pool worker.WorkerPool) {
	b.workerPool = pool
}

// WorkerPool 返回 worker 池
func (b *BaseService) WorkerPool() worker.WorkerPool {
	return b.workerPool
}

// StoreResult 存储初始结果
func (b *BaseService) StoreResult(taskID string, result *CrawlerResult) {
	b.results.Store(taskID, result)
}

// GetTask 获取任务结果
func (b *BaseService) GetTask(taskID string) (*CrawlerResult, error) {
	value, ok := b.results.Load(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	return value.(*CrawlerResult), nil
}

// DeleteTask 删除任务
func (b *BaseService) DeleteTask(taskID string) {
	b.results.Delete(taskID)
}

// GetAllTasks 获取所有任务
func (b *BaseService) GetAllTasks() []*CrawlerResult {
	tasks := make([]*CrawlerResult, 0)
	b.results.Range(func(_, value any) bool {
		tasks = append(tasks, value.(*CrawlerResult))
		return true
	})
	return tasks
}

// UpdateResult 线程安全地更新任务结果
func (b *BaseService) UpdateResult(taskID string, fn func(*CrawlerResult)) {
	b.resultMu.Lock()
	defer b.resultMu.Unlock()
	if value, ok := b.results.Load(taskID); ok {
		result := value.(*CrawlerResult)
		fn(result)
		b.results.Store(taskID, result)
	}
}

// GetStats 获取通用队列与指标统计
func (b *BaseService) GetStats() map[string]any {
	stats := map[string]any{
		"queue_size":      0,
		"queue_capacity":  0,
		"available_slots": 0,
		"usage_percent":   0.0,
	}
	if b.workerPool != nil {
		queueStats := b.workerPool.GetQueueStats()
		stats["queue_size"] = queueStats.QueueSize
		stats["queue_capacity"] = queueStats.BufferSize
		stats["available_slots"] = queueStats.AvailableSlots
		stats["usage_percent"] = queueStats.UsagePercent
	}

	statusCount := make(map[string]int)
	b.results.Range(func(_, value any) bool {
		statusCount[string(value.(*CrawlerResult).Status)]++
		return true
	})
	stats["status_count"] = statusCount

	if b.workerPool != nil {
		if metrics := b.workerPool.GetMetrics(); metrics != nil {
			snapshot := metrics.GetSnapshot()
			stats["total_submitted"] = snapshot.TotalSubmitted
			stats["total_processed"] = snapshot.TotalProcessed
			stats["total_succeeded"] = snapshot.TotalSucceeded
			stats["total_failed"] = snapshot.TotalFailed
			stats["total_panicked"] = snapshot.TotalPanicked
			stats["queue_full_count"] = snapshot.QueueFullCount
			stats["success_rate"] = snapshot.SuccessRate()
			stats["failure_rate"] = snapshot.FailureRate()
			stats["panic_rate"] = snapshot.PanicRate()
			stats["uptime"] = snapshot.Uptime.String()
		}
	}

	return stats
}

// IsReady 检查服务是否就绪
func (b *BaseService) IsReady() bool {
	return b.workerPool.AvailableSlots() > 0
}

// IsHealthy 检查服务是否健康
func (b *BaseService) IsHealthy() bool {
	return true
}
