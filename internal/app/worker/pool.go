// Package worker 提供工作池实现，用于并发处理任务
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/domain/model"
	"time"
)

// Pool 工作池实现
type Pool struct {
	processor          Processor
	concurrency        int
	bufferSize         int
	jobQueue           chan WorkerJob
	workers            []*Worker
	wg                 sync.WaitGroup
	closed             bool
	mu                 sync.RWMutex
	completionNotifier TaskCompletionNotifier // 任务完成通知器
}

// Worker 工作协程
type Worker struct {
	id        int
	pool      *Pool
	jobQueue  <-chan WorkerJob
	processor Processor
}

// NewPool 创建新的工作池
// 参数:
//   - proc: 任务处理器，用于处理具体的任务逻辑
//   - workerCfg: 工作池配置，包含并发数和缓冲区大小
//
// 返回值:
//   - *Pool: 工作池实例
func NewPool(proc Processor, workerCfg config.WorkerConfig) *Pool {
	return &Pool{
		processor:          proc,
		concurrency:        workerCfg.Concurrency,
		bufferSize:         workerCfg.BufferSize,
		jobQueue:           make(chan WorkerJob, workerCfg.BufferSize),
		workers:            make([]*Worker, 0, workerCfg.Concurrency),
		completionNotifier: nil,
	}
}

// SetCompletionNotifier 设置任务完成通知器
// 用于在任务完成时接收通知，通常用于清理资源或更新状态
// 参数:
//   - notifier: 任务完成通知器接口
func (p *Pool) SetCompletionNotifier(notifier TaskCompletionNotifier) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completionNotifier = notifier
	logger.GetGlobalLogger("worker.pool").Info("已设置任务完成通知器")
}

// Start 启动工作池
// 创建指定数量的工作协程并开始处理任务队列中的任务
// 参数:
//   - ctx: 上下文，用于控制工作池的生命周期
func (p *Pool) Start(ctx context.Context) {
	log := logger.GetGlobalLogger("worker.pool")
	log.WithFields(map[string]interface{}{
		"concurrency": p.concurrency,
		"buffer_size": p.bufferSize,
	}).Info("启动工作池")

	for i := 0; i < p.concurrency; i++ {
		worker := &Worker{
			id:        i,
			pool:      p,
			jobQueue:  p.jobQueue,
			processor: p.processor,
		}
		p.workers = append(p.workers, worker)
		p.wg.Add(1)
		go worker.Run(ctx, &p.wg)
	}

	log.WithField("worker_count", p.concurrency).Info("工作池已启动")
}

// Stop 停止工作池
// 优雅关闭工作池，等待所有正在处理的任务完成
// 参数:
//   - ctx: 上下文，用于控制关闭超时
func (p *Pool) Stop(ctx context.Context) {
	log := logger.GetGlobalLogger("worker.pool")
	log.Info("开始优雅关闭工作池")

	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.jobQueue)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("等待工作池完成goroutine panic recovered")
			}
		}()

		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("所有工作协程已停止")
	case <-ctx.Done():
		log.Warn("等待工作协程停止超时")
	}

	log.Info("工作池已完成优雅关闭")
}

// Submit 提交任务
// 将任务提交到工作池的任务队列中等待处理
// 参数:
//   - job: 要处理的工作任务
//
// 返回值:
//   - error: 提交失败时返回错误，成功时返回nil
func (p *Pool) Submit(job WorkerJob) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return ErrPoolClosed
	}

	select {
	case p.jobQueue <- job:
		return nil
	default:
		logger.GetGlobalLogger("worker.pool").WithFields(map[string]interface{}{
			"tenant_id": job.TenantID,
			"shop_id":   job.ShopID,
		}).Warn("工作池队列已满，任务提交失败")
		return ErrQueueFull
	}
}

// AvailableSlots 返回可用槽位数
// 计算当前任务队列中剩余的可用空间
// 返回值:
//   - int: 可用的任务槽位数量
func (p *Pool) AvailableSlots() int {
	return p.bufferSize - len(p.jobQueue)
}

// Run 工作协程运行
func (w *Worker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	log := logger.GetGlobalLogger("worker.pool")
	log.WithField("worker_id", w.id).Info("工作协程已启动")

	for {
		select {
		case <-ctx.Done():
			log.WithField("worker_id", w.id).Info("工作协程正在停止")
			return
		case job, ok := <-w.jobQueue:
			if !ok {
				log.WithField("worker_id", w.id).Info("工作协程任务队列已关闭")
				return
			}

			// 使用 defer 和 recover 确保 panic 不会导致工作协程崩溃
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(map[string]interface{}{
							"worker_id": w.id,
							"panic":     r,
						}).Error("工作协程发生panic")

						// 打印堆栈跟踪
						buf := make([]byte, 4096)
						n := runtime.Stack(buf, false)
						log.WithField("stack_trace", string(buf[:n])).Error("堆栈跟踪")

						// 尝试解析任务以记录更多信息
						var task model.Task
						if err := json.Unmarshal([]byte(job.TaskData), &task); err == nil {
							log.WithFields(map[string]interface{}{
								logger.FieldTaskID:    task.ID,
								logger.FieldProductID: task.ProductID,
							}).Error("Panic发生在任务")
						}
					}
				}()

				var task model.Task
				if err := json.Unmarshal([]byte(job.TaskData), &task); err != nil {
					log.WithFields(map[string]interface{}{
						"worker_id": w.id,
						"task_data": job.TaskData,
					}).WithError(err).Error("工作协程解析任务数据失败")
					return
				}

				// 设置任务处理超时时间为15分钟（任务包含AI处理、图片上传等耗时操作）
				processCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
				defer cancel()

				log.WithFields(map[string]interface{}{
					"worker_id":           w.id,
					logger.FieldTaskID:    task.ID,
					logger.FieldProductID: task.ProductID,
				}).Info("工作协程开始处理任务")

				// 确保任务处理完成后清理（无论成功或失败）
				defer func() {
					// 通知任务获取器移除该任务
					if w.pool.completionNotifier != nil {
						w.pool.completionNotifier.OnTaskCompleted(task.ID)
					}
				}()

				if err := w.processor.ProcessTask(processCtx, &task); err != nil {
					log.WithFields(map[string]interface{}{
						"worker_id":        w.id,
						logger.FieldTaskID: task.ID,
					}).WithError(err).Error("工作协程处理任务失败")
				} else {
					log.WithFields(map[string]interface{}{
						"worker_id":        w.id,
						logger.FieldTaskID: task.ID,
					}).Info("工作协程任务处理完成")
				}
			}()
		}
	}
}

// GetQueueStats 获取队列统计信息
// 返回当前工作池的队列使用情况统计
// 返回值:
//   - QueueStats: 包含队列大小、缓冲区大小、可用槽位等信息
func (p *Pool) GetQueueStats() QueueStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	queueLen := len(p.jobQueue)
	usagePercent := 0.0
	if p.bufferSize > 0 {
		usagePercent = float64(queueLen) / float64(p.bufferSize) * 100
	}

	return QueueStats{
		QueueSize:      queueLen,
		BufferSize:     p.bufferSize,
		AvailableSlots: p.bufferSize - queueLen,
		UsagePercent:   usagePercent,
	}
}

// 错误定义
var (
	ErrQueueFull  = fmt.Errorf("工作队列已满")
	ErrPoolClosed = fmt.Errorf("工作池已关闭")
)
