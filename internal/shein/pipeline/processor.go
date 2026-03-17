package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/app/processor"
	"task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	types "task-processor/internal/model"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	commonPipeline "task-processor/internal/pipeline"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

// amazonCrawler 定义 SHEIN 处理器对 Amazon 爬虫的依赖（消费者定义接口原则）。
type amazonCrawler interface {
	Process(url string, zipcode string) (*model.Product, error)
}

// SheinProcessor SHEIN任务处理器
type SheinProcessor struct {
	*processor.BaseProcessor                         // 继承基础处理器
	amazonProcessor          amazonCrawler           // SHEIN特定：共享的Amazon爬虫（接口）
	rabbitmqClient           *rabbitmq.Client        // RabbitMQ客户端（用于分布式爬虫）
	taskHandler              *TaskHandler            // SHEIN特定：任务处理器
	pipeline                 commonPipeline.Pipeline // SHEIN特定：处理管道
}

// NewSheinProcessor 创建SHEIN处理器（参考Temu实现）
// 返回 error 而不是 panic，让调用方决定如何处理错误
func NewSheinProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedAmazonProcessor amazonCrawler, rabbitmqClient *rabbitmq.Client) (*SheinProcessor, error) {
	// ManagementClient必须由调用方提供（共享实例）
	if managementClient == nil {
		logger.Error("[SHEIN] ManagementClient不能为空，必须使用共享实例")
		return nil, fmt.Errorf("managementClient不能为空")
	}

	// 如果提供了Amazon处理器，记录日志
	if sharedAmazonProcessor != nil {
		logger.Info("[SHEIN] 使用共享的Amazon爬虫实例")
	}

	// 记录RabbitMQ客户端状态
	if rabbitmqClient != nil {
		logger.Info("[SHEIN] ✅ 使用RabbitMQ客户端，将启用分布式爬虫")
	} else {
		logger.Warn("[SHEIN] ⚠️ 未提供RabbitMQ客户端，将使用本地爬虫")
	}

	logger.Info("[SHEIN] 使用共享的管理客户端")

	// 创建基础处理器
	baseProcessor := processor.NewBaseProcessor(ctx, &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: managementClient,
		Logger:           logger,
		Platform:         "SHEIN",
	})

	p := &SheinProcessor{
		BaseProcessor:   baseProcessor,
		amazonProcessor: sharedAmazonProcessor,
		rabbitmqClient:  rabbitmqClient,
	}

	// 创建 WorkerPool（内部管理）
	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)

	// 初始化SHEIN特定组件
	p.taskHandler = NewTaskHandler(p)
	p.pipeline = p.buildPipeline()

	return p, nil
}

// buildPipeline 构建管道（统一方法，参考TEMU）
func (p *SheinProcessor) buildPipeline() commonPipeline.Pipeline {
	// 使用现有的管道创建函数
	sheinPipeline := CreateTaskProcessingPipeline(p, p.GetConfig())

	// 将 SHEIN 特定的管道转换为通用管道
	// 这里需要适配器模式，但为了快速修复，我们直接返回一个新的通用管道
	pipeline := commonPipeline.NewPipeline("SHEIN产品处理管道")

	// 将 SHEIN 的处理器适配到通用管道
	for _, handler := range sheinPipeline.Handlers() {
		// 创建适配器包装 SHEIN 处理器
		adapter := NewSheinHandlerAdapter(handler)
		pipeline.AddHandler(adapter)
	}

	return pipeline
}

// Start 启动任务处理器
func (p *SheinProcessor) Start(ctx context.Context) error {
	// 启动基础组件
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	p.GetLogger().Info("[SHEIN] 任务处理器启动完成")
	return nil
}

// ProcessTask 处理任务 - 实现worker.Processor接口
func (p *SheinProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	// 解析任务数据
	var task types.Task
	if err := jsonx.UnmarshalString(job.TaskData, &task, "解析任务数据失败"); err != nil {
		return err
	}

	return p.taskHandler.ProcessTask(ctx, task, p.pipeline)
}

// GetAmazonProcessor 获取共享的Amazon处理器
func (p *SheinProcessor) GetAmazonProcessor() amazonCrawler {
	return p.amazonProcessor
}

// CreateTaskSubmitter 创建任务提交器（使用WorkerPool）
func (p *SheinProcessor) CreateTaskSubmitter(workerPool worker.WorkerPool) task.TaskSubmitter {
	return NewSheinTaskSubmitter(workerPool)
}

// Close 关闭处理器
func (p *SheinProcessor) Close(ctx context.Context) {
	p.GetLogger().Info("[SHEIN] 关闭任务处理器")

	// 关闭基础组件
	p.CloseBase(ctx)

	p.GetLogger().Info("[SHEIN] 任务处理器已关闭")
}

