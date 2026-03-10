package temu

import (
	"context"
	"fmt"
	"task-processor/internal/app/processor"
	"task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// TemuProcessor TEMU平台处理器
type TemuProcessor struct {
	*processor.BaseProcessor                         // 继承基础处理器
	amazonProcessor          *amazon.AmazonProcessor // TEMU特定：Amazon处理器
	rabbitmqClient           *rabbitmq.Client        // RabbitMQ客户端（用于分布式爬虫）
	taskHandler              *TaskHandler            // TEMU特定：任务处理器
	pipelineExecutor         *TemuPipelineExecutor   // TEMU特定：强类型管道执行器
}

// NewTemuProcessor 创建TEMU处理器
// 返回 error 而不是 panic，让调用方决定如何处理错误
func NewTemuProcessor(ctx context.Context, cfg *config.Config, loggerInstance *logrus.Logger, managementClient *management.ClientManager, sharedAmazonProcessor *amazon.AmazonProcessor, rabbitmqClient *rabbitmq.Client) (*TemuProcessor, error) {
	log := logger.GetGlobalLogger("temu_processor").WithField(logger.FieldPlatform, "temu")

	// ManagementClient必须由调用方提供（共享实例）
	if managementClient == nil {
		log.Error("ManagementClient不能为空，必须使用共享实例")
		return nil, fmt.Errorf("managementClient不能为空")
	}

	// Amazon处理器必须使用共享实例（资源优化）
	if sharedAmazonProcessor == nil {
		log.Error("SharedAmazonProcessor不能为空，必须使用共享实例")
		return nil, fmt.Errorf("sharedAmazonProcessor不能为空")
	}

	log.Info("使用共享的Amazon爬虫实例和管理客户端")

	// 记录RabbitMQ客户端状态
	if rabbitmqClient != nil {
		log.Info("✅ 使用RabbitMQ客户端，将启用分布式爬虫")
	} else {
		log.Warn("⚠️ 未提供RabbitMQ客户端，将使用本地爬虫")
	}

	// 创建基础处理器
	baseProcessor := processor.NewBaseProcessor(ctx, &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: managementClient,
		Logger:           loggerInstance,
		Platform:         "TEMU",
	})

	p := &TemuProcessor{
		BaseProcessor:   baseProcessor,
		amazonProcessor: sharedAmazonProcessor,
		rabbitmqClient:  rabbitmqClient,
	}

	// 创建 WorkerPool（内部管理）
	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)

	// 初始化任务处理器
	p.taskHandler = NewTaskHandler(p)

	// 使用统一的管道构建方法
	p.pipelineExecutor = p.buildPipelineExecutor()

	return p, nil
}

// ProcessTask 处理TEMU任务 - 实现worker.Processor接口
func (p *TemuProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	// 解析任务数据
	var task model.Task
	if err := jsonutil.UnmarshalString(job.TaskData, &task, "解析任务数据失败"); err != nil {
		return err
	}

	log := p.GetLogger()
	log.WithFields(logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
		logger.FieldStoreID:   task.StoreID,
	}).Info("开始处理任务")

	// TEMU特定的任务处理逻辑
	if err := p.processTemuProduct(ctx, task); err != nil {
		log.WithError(err).WithField(logger.FieldTaskID, task.ID).Error("处理产品失败")
		return err
	}

	log.WithField(logger.FieldTaskID, task.ID).Info("任务处理完成")
	return nil
}

// processTemuProduct 处理TEMU产品
func (p *TemuProcessor) processTemuProduct(ctx context.Context, task model.Task) error {
	log := p.GetLogger()
	log.WithField(logger.FieldProductID, task.ProductID).Info("处理产品")

	// 使用任务处理器处理任务
	if err := p.taskHandler.ProcessTask(ctx, task, p.pipelineExecutor); err != nil {
		return fmt.Errorf("任务处理失败: %w", err)
	}

	log.WithField(logger.FieldProductID, task.ProductID).Info("产品处理完成")
	return nil
}

// buildPipelineExecutor 构建完整的TEMU强类型管道执行器
func (p *TemuProcessor) buildPipelineExecutor() *TemuPipelineExecutor {
	// 使用专门的管道构建器，避免循环导入
	builder := NewPipelineBuilder(p)
	return builder.BuildPipeline()
}

// Start 启动TEMU处理器
func (p *TemuProcessor) Start(ctx context.Context) error {
	// 启动基础组件
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	p.GetLogger().Info("任务处理器启动完成")
	return nil
}

// GetAmazonProcessor 获取共享的Amazon处理器
func (p *TemuProcessor) GetAmazonProcessor() any {
	return p.amazonProcessor
}

// CreateTaskSubmitter 创建任务提交器（使用WorkerPool）
func (p *TemuProcessor) CreateTaskSubmitter(workerPool worker.WorkerPool) task.TaskSubmitter {
	return NewTemuTaskSubmitter(workerPool)
}

// Close 关闭处理器
func (p *TemuProcessor) Close(ctx context.Context) {
	log := p.GetLogger()
	log.Info("关闭TEMU任务处理器")

	// 关闭基础组件
	p.CloseBase(ctx)

	// 关闭Amazon处理器
	if p.amazonProcessor != nil {
		p.amazonProcessor.Shutdown()
	}

	log.Info("任务处理器已关闭")
}
