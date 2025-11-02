package processor

import (
	"context"
	"task-processor/common/config"
	"task-processor/common/types"
	"time"

	"github.com/sirupsen/logrus"
)

// Processor 任务处理器接口
type Processor interface {
	Start(ctx context.Context) error
	ProcessTask(ctx context.Context, task types.Task) error
	Close()
}

// BaseProcessor 基础处理器实现
type BaseProcessor struct {
	config      *config.Config
	taskFetcher TaskFetcher
	workerPool  WorkerPool
}

// TaskFetcher 任务获取器接口
type TaskFetcher interface {
	Start(ctx context.Context)
	GetPendingTasks(maxTasks int) ([]string, error)
}

// WorkerPool 工作池接口
type WorkerPool interface {
	Start(ctx context.Context)
	Stop(ctx context.Context)
	Submit(job WorkerJob) error
	AvailableSlots() int
}

// WorkerJob 工作任务
type WorkerJob struct {
	TenantID string
	ShopID   string
	TaskData string
}

// NewBaseProcessor 创建基础处理器
func NewBaseProcessor(cfg *config.Config) *BaseProcessor {
	return &BaseProcessor{
		config: cfg,
	}
}

// Start 启动处理器
func (p *BaseProcessor) Start(ctx context.Context) error {
	logrus.Info("启动基础任务处理器")

	if p.workerPool != nil {
		p.workerPool.Start(ctx)
	}

	if p.taskFetcher != nil {
		go p.taskFetcher.Start(ctx)
	}

	return nil
}

// ProcessTask 处理任务
func (p *BaseProcessor) ProcessTask(ctx context.Context, task types.Task) error {
	logrus.Infof("处理任务: ID=%s, ProductID=%s", task.ID, task.ProductID)

	// 基础实现，子类应该重写此方法
	time.Sleep(1 * time.Second)

	logrus.Infof("任务处理完成: ID=%s", task.ID)
	return nil
}

// Close 关闭处理器
func (p *BaseProcessor) Close() {
	logrus.Info("关闭基础任务处理器")
}

// SetTaskFetcher 设置任务获取器
func (p *BaseProcessor) SetTaskFetcher(fetcher TaskFetcher) {
	p.taskFetcher = fetcher
}

// SetWorkerPool 设置工作池
func (p *BaseProcessor) SetWorkerPool(pool WorkerPool) {
	p.workerPool = pool
}
