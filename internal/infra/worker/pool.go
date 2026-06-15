// Package worker 提供工作池实现，用于并发处理任务
package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/recovery"
	"time"
)

// Pool 工作池实现
type Pool struct {
	processor  Processor
	config     PoolConfig // 使用新的配置结构
	jobQueue   chan WorkerJob
	workers    []*Worker
	wg         sync.WaitGroup
	closed     bool
	mu         sync.RWMutex
	jobHandler JobHandler // 任务处理钩子
	metrics    *Metrics   // 指标收集器

	lastQueueFullLogUnixNano int64
	suppressedQueueFullLogs  uint64
}

const queueFullLogInterval = 30 * time.Second

// NewPool 创建新的工作池（兼容旧版本）
// 参数:
//   - proc: 任务处理器，用于处理具体的任务逻辑
//   - workerCfg: 工作池配置，包含并发数和缓冲区大小
//
// 返回值:
//   - *Pool: 工作池实例
func NewPool(proc Processor, workerCfg config.WorkerConfig) *Pool {
	// 从默认配置开始
	poolCfg := DefaultPoolConfig()

	// 覆盖从配置文件读取的值
	poolCfg.Concurrency = workerCfg.Concurrency
	poolCfg.BufferSize = workerCfg.BufferSize

	return NewPoolWithConfig(proc, poolCfg)
}

// NewPoolWithConfig 使用新配置创建工作池
// 参数:
//   - proc: 任务处理器
//   - cfg: 工作池配置
//
// 返回值:
//   - *Pool: 工作池实例
func NewPoolWithConfig(proc Processor, cfg PoolConfig) *Pool {
	log := logger.GetGlobalLogger("worker.pool")

	// 验证配置并记录警告
	if err := cfg.Validate(); err != nil {
		log.Warn(err.Error())
	}

	pool := &Pool{
		processor:  proc,
		config:     cfg,
		jobQueue:   make(chan WorkerJob, cfg.BufferSize),
		workers:    make([]*Worker, 0, cfg.Concurrency),
		jobHandler: nil,
	}

	// 如果启用指标收集
	if cfg.EnableMetrics {
		pool.metrics = NewMetrics()
		log.Debug("已启用指标收集")
	}

	return pool
}

// SetJobHandler 设置任务处理钩子
// 用于在任务处理的各个阶段接收通知
// 参数:
//   - handler: 任务处理钩子接口
func (p *Pool) SetJobHandler(handler JobHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.jobHandler = handler
	logger.GetGlobalLogger("worker.pool").Debug("已设置任务处理钩子")
}

// getJobHandler 线程安全地获取任务处理钩子
func (p *Pool) getJobHandler() JobHandler {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.jobHandler
}

// Start 启动工作池
// 创建指定数量的工作协程并开始处理任务队列中的任务
// 参数:
//   - ctx: 上下文，用于控制工作池的生命周期
func (p *Pool) Start(ctx context.Context) {
	log := logger.GetGlobalLogger("worker.pool")
	log.WithFields(map[string]any{
		"concurrency": p.config.Concurrency,
		"buffer_size": p.config.BufferSize,
	}).Info("启动工作池")

	for i := 0; i < p.config.Concurrency; i++ {
		worker := &Worker{
			id:        i,
			pool:      p,
			jobQueue:  p.jobQueue,
			processor: p.processor,
			logger:    log.WithField("worker_id", i),
		}
		p.workers = append(p.workers, worker)
		p.wg.Add(1)
		go worker.Run(ctx, &p.wg)
	}

	log.WithField("worker_count", p.config.Concurrency).Info("工作池已启动")
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
		defer recovery.Recover("等待工作池完成", log)
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

	// 记录提交
	if p.metrics != nil {
		p.metrics.RecordSubmit()
	}

	select {
	case p.jobQueue <- job:
		return nil
	default:
		// 记录队列满
		if p.metrics != nil {
			p.metrics.RecordQueueFull()
		}

		p.logQueueFull(job)
		return ErrQueueFull
	}
}

func (p *Pool) logQueueFull(job WorkerJob) {
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&p.lastQueueFullLogUnixNano)

	if last == 0 || time.Duration(now-last) >= queueFullLogInterval {
		if atomic.CompareAndSwapInt64(&p.lastQueueFullLogUnixNano, last, now) {
			suppressed := atomic.SwapUint64(&p.suppressedQueueFullLogs, 0)
			log := logger.GetGlobalLogger("worker.pool").WithFields(map[string]any{
				"tenant_id":   job.TenantID,
				"shop_id":     job.ShopID,
				"queue_size":  len(p.jobQueue),
				"buffer_size": p.config.BufferSize,
			})
			if suppressed > 0 {
				log = log.WithField("suppressed_count", suppressed)
			}
			log.Warn("工作池队列已满，任务提交失败")
			return
		}
	}

	atomic.AddUint64(&p.suppressedQueueFullLogs, 1)
}

// AvailableSlots 返回可用槽位数
// 计算当前任务队列中剩余的可用空间
// 返回值:
//   - int: 可用的任务槽位数量
func (p *Pool) AvailableSlots() int {
	return p.config.BufferSize - len(p.jobQueue)
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
	if p.config.BufferSize > 0 {
		usagePercent = float64(queueLen) / float64(p.config.BufferSize) * 100
	}

	return QueueStats{
		QueueSize:      queueLen,
		BufferSize:     p.config.BufferSize,
		AvailableSlots: p.config.BufferSize - queueLen,
		UsagePercent:   usagePercent,
	}
}

// GetMetrics 获取指标收集器
// 返回值:
//   - *Metrics: 指标收集器，如果未启用则返回nil
func (p *Pool) GetMetrics() *Metrics {
	return p.metrics
}

// 错误定义
var (
	ErrQueueFull  = fmt.Errorf("工作队列已满")
	ErrPoolClosed = fmt.Errorf("工作池已关闭")
)
