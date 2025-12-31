// Package worker 提供工作池实现，用于并发处理任务
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"time"

	"github.com/sirupsen/logrus"
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
	logrus.Info("已设置任务完成通知器")
}

// Start 启动工作池
// 创建指定数量的工作协程并开始处理任务队列中的任务
// 参数:
//   - ctx: 上下文，用于控制工作池的生命周期
func (p *Pool) Start(ctx context.Context) {
	logrus.Infof("启动工作池: 并发数=%d, 缓冲区大小=%d", p.concurrency, p.bufferSize)

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

	logrus.Infof("工作池已启动，%d 个工作协程就绪", p.concurrency)
}

// Stop 停止工作池
// 优雅关闭工作池，等待所有正在处理的任务完成
// 参数:
//   - ctx: 上下文，用于控制关闭超时
func (p *Pool) Stop(ctx context.Context) {
	logrus.Info("开始优雅关闭工作池")

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
				logrus.Errorf("等待工作池完成goroutine panic recovered: %v", r)
			}
		}()

		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logrus.Info("所有工作协程已停止")
	case <-ctx.Done():
		logrus.Warn("等待工作协程停止超时")
	}

	logrus.Info("工作池已完成优雅关闭")
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
		logrus.Warnf("工作池队列已满，任务提交失败: TenantID=%s, ShopID=%s", job.TenantID, job.ShopID)
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

	logrus.Infof("工作协程 %d 已启动", w.id)

	for {
		select {
		case <-ctx.Done():
			logrus.Infof("工作协程 %d 正在停止", w.id)
			return
		case job, ok := <-w.jobQueue:
			if !ok {
				logrus.Infof("工作协程 %d 任务队列已关闭", w.id)
				return
			}

			// 使用 defer 和 recover 确保 panic 不会导致工作协程崩溃
			func() {
				defer func() {
					if r := recover(); r != nil {
						logrus.Errorf("工作协程 %d 发生 panic: %v", w.id, r)

						// 打印堆栈跟踪
						buf := make([]byte, 4096)
						n := runtime.Stack(buf, false)
						logrus.Errorf("堆栈跟踪:\n%s", string(buf[:n]))

						// 尝试解析任务以记录更多信息
						var task model.Task
						if err := json.Unmarshal([]byte(job.TaskData), &task); err == nil {
							logrus.Errorf("Panic 发生在任务: TaskID=%d, ProductID=%s", task.ID, task.ProductID)
						}
					}
				}()

				var task model.Task
				if err := json.Unmarshal([]byte(job.TaskData), &task); err != nil {
					logrus.Errorf("工作协程 %d 解析任务数据失败: %v, 原始数据: %s", w.id, err, job.TaskData)
					return
				}

				// 设置任务处理超时时间为15分钟（任务包含AI处理、图片上传等耗时操作）
				processCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
				defer cancel()

				logrus.Infof("工作协程 %d 开始处理任务: TaskID=%d, ProductID=%s", w.id, task.ID, task.ProductID)

				// 确保任务处理完成后清理（无论成功或失败）
				defer func() {
					// 通知任务获取器移除该任务
					if w.pool.completionNotifier != nil {
						w.pool.completionNotifier.OnTaskCompleted(task.ID)
					}
				}()

				if err := w.processor.ProcessTask(processCtx, &task); err != nil {
					logrus.Errorf("工作协程 %d 处理任务失败: TaskID=%d, Error=%v", w.id, task.ID, err)
				} else {
					logrus.Infof("工作协程 %d 任务处理完成: TaskID=%d", w.id, task.ID)
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
