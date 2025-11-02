package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"task-processor/common/config"
	"task-processor/common/processor"
	"task-processor/common/types"
	"time"

	"github.com/sirupsen/logrus"
)

// Pool 工作池实现
type Pool struct {
	processor   processor.Processor
	concurrency int
	bufferSize  int
	jobQueue    chan processor.WorkerJob
	workers     []*Worker
	wg          sync.WaitGroup
}

// Worker 工作协程
type Worker struct {
	id        int
	pool      *Pool
	jobQueue  <-chan processor.WorkerJob
	processor processor.Processor
}

// NewPool 创建新的工作池
func NewPool(proc processor.Processor, workerCfg config.WorkerConfig) *Pool {
	return &Pool{
		processor:   proc,
		concurrency: workerCfg.Concurrency,
		bufferSize:  workerCfg.BufferSize,
		jobQueue:    make(chan processor.WorkerJob, workerCfg.BufferSize),
		workers:     make([]*Worker, 0, workerCfg.Concurrency),
	}
}

// Start 启动工作池
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
func (p *Pool) Stop(ctx context.Context) {
	logrus.Info("开始优雅关闭工作池")

	close(p.jobQueue)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logrus.Info("所有工作协程已停止")
	case <-ctx.Done():
		logrus.Warn("等待工作协程停止超时")
	}

	remainingTasks := len(p.jobQueue)
	if remainingTasks > 0 {
		logrus.Warnf("jobQueue 中还有 %d 个未处理的任务", remainingTasks)
	}

	logrus.Info("工作池已完成优雅关闭")
}

// Submit 提交任务
func (p *Pool) Submit(job processor.WorkerJob) error {
	select {
	case p.jobQueue <- job:
		return nil
	default:
		logrus.Warnf("工作池队列已满，任务提交失败: TenantID=%s, ShopID=%s", job.TenantID, job.ShopID)
		return ErrQueueFull
	}
}

// AvailableSlots 返回可用槽位数
func (p *Pool) AvailableSlots() int {
	return p.bufferSize - len(p.jobQueue)
}

// Run 工作协程运行
func (w *Worker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	logrus.Printf("工作协程 %d 已启动", w.id)

	for {
		select {
		case <-ctx.Done():
			logrus.Printf("工作协程 %d 正在停止", w.id)
			return
		case job, ok := <-w.jobQueue:
			if !ok {
				logrus.Printf("工作协程 %d 任务队列已关闭", w.id)
				return
			}

			var task types.Task
			if err := json.Unmarshal([]byte(job.TaskData), &task); err != nil {
				logrus.Errorf("工作协程 %d 解析任务数据失败: %v", w.id, err)
				continue
			}

			processCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)

			logrus.Infof("工作协程 %d 开始处理任务: TaskID=%s, ProductID=%s", w.id, task.ID, task.ProductID)

			if err := w.processor.ProcessTask(processCtx, task); err != nil {
				logrus.Errorf("工作协程 %d 处理任务失败: TaskID=%s, Error=%v", w.id, task.ID, err)
			} else {
				logrus.Infof("工作协程 %d 任务处理完成: TaskID=%s", w.id, task.ID)
			}

			cancel()
		}
	}
}

// ErrQueueFull 队列已满错误
var ErrQueueFull = fmt.Errorf("工作队列已满")
