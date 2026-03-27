// Package distributed 提供等待任务注册表
package distributed

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PendingRegistry 管理所有等待结果的任务（可独立测试）
type PendingRegistry struct {
	tasks   map[string]*PendingTask
	mu      sync.RWMutex
	timeout time.Duration
}

// NewPendingRegistry 创建注册表
func NewPendingRegistry(timeout time.Duration) *PendingRegistry {
	return &PendingRegistry{
		tasks:   make(map[string]*PendingTask),
		timeout: timeout,
	}
}

// Register 注册一个等待任务，返回 PendingTask
func (r *PendingRegistry) Register(ctx context.Context, taskID string) *PendingTask {
	taskCtx, cancel := context.WithTimeout(ctx, r.timeout)
	pt := &PendingTask{
		TaskID:     taskID,
		ResultChan: make(chan *CrawlResult, 1),
		CreatedAt:  time.Now(),
		Context:    taskCtx,
		Cancel:     cancel,
	}
	r.mu.Lock()
	r.tasks[pt.TaskID] = pt
	r.mu.Unlock()

	dl, _ := taskCtx.Deadline()
	fmt.Printf("[PendingRegistry] 注册任务: TaskID=%s, 超时时间=%v, deadline=%s, 当前pending数=%d\n",
		taskID, r.timeout, dl.Format("15:04:05"), len(r.tasks))
	return pt
}

// Remove 移除并取消一个等待任务
func (r *PendingRegistry) Remove(taskID string) {
	r.mu.Lock()
	if pt, ok := r.tasks[taskID]; ok {
		pt.Cancel()
		delete(r.tasks, taskID)
	}
	r.mu.Unlock()
}

// Deliver 将结果投递给等待的任务，返回是否成功找到任务
func (r *PendingRegistry) Deliver(result *CrawlResult) bool {
	taskID := result.TaskID

	r.mu.RLock()
	pt, exists := r.tasks[taskID]
	r.mu.RUnlock()

	if !exists {
		return false
	}

	// 先写入 channel，再删除任务和 cancel context
	// 避免 cancel 后 Wait 里 select 随机选中 Done() 而非 ResultChan
	select {
	case pt.ResultChan <- result:
	default:
		// channel 已满（不应发生，缓冲为 1）
	}

	r.mu.Lock()
	delete(r.tasks, taskID)
	r.mu.Unlock()
	pt.Cancel()
	return true
}

// Wait 等待指定任务的结果
func (r *PendingRegistry) Wait(pt *PendingTask) (*CrawlResult, error) {
	// 优先检查 channel，避免 context cancel 和 channel 同时就绪时随机选中 Done()
	select {
	case result := <-pt.ResultChan:
		elapsed := time.Since(pt.CreatedAt)
		fmt.Printf("[PendingRegistry] 任务立即完成(已在channel): TaskID=%s, 耗时=%.1fs\n", pt.TaskID, elapsed.Seconds())
		return result, nil
	default:
	}
	select {
	case result := <-pt.ResultChan:
		elapsed := time.Since(pt.CreatedAt)
		fmt.Printf("[PendingRegistry] 任务完成: TaskID=%s, 耗时=%.1fs\n", pt.TaskID, elapsed.Seconds())
		return result, nil
	case <-pt.Context.Done():
		elapsed := time.Since(pt.CreatedAt)
		dl, _ := pt.Context.Deadline()
		fmt.Printf("[PendingRegistry] 任务超时: TaskID=%s, 等待耗时=%.1fs, deadline=%s, ctxErr=%v\n",
			pt.TaskID, elapsed.Seconds(), dl.Format("15:04:05"), pt.Context.Err())
		r.Remove(pt.TaskID)
		return nil, fmt.Errorf("爬虫任务超时: TaskID=%s", pt.TaskID)
	}
}

// Len 返回当前等待任务数量
func (r *PendingRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tasks)
}
