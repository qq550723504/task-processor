package temu

import (
	"context"
	"fmt"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/management"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"task-processor/internal/task"
	"task-processor/internal/worker"

	"github.com/sirupsen/logrus"
)

// TemuProcessor TEMU平台处理器
type TemuProcessor struct {
	*worker.BaseProcessor                         // 继承基础处理器
	amazonProcessor       *amazon.AmazonProcessor // TEMU特定：Amazon处理器
	taskHandler           *TaskHandler            // TEMU特定：任务处理器
	pipelineExecutor      *TemuPipelineExecutor   // TEMU特定：强类型管道执行器
}

// NewTemuProcessor 创建TEMU处理器
func NewTemuProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedAmazonProcessor *amazon.AmazonProcessor) *TemuProcessor {
	// ManagementClient必须由调用方提供（共享实例）
	if managementClient == nil {
		logger.Error("[TEMU] ManagementClient不能为空，必须使用共享实例")
		panic("ManagementClient不能为空")
	}

	// Amazon处理器必须使用共享实例（资源优化）
	if sharedAmazonProcessor == nil {
		logger.Error("[TEMU] SharedAmazonProcessor不能为空，必须使用共享实例")
		panic("SharedAmazonProcessor不能为空")
	}

	logger.Info("[TEMU] 使用共享的Amazon爬虫实例和管理客户端")

	// 创建基础处理器
	baseProcessor := worker.NewBaseProcessor(ctx, &worker.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: managementClient,
		Logger:           logger,
		Platform:         "TEMU",
	})

	p := &TemuProcessor{
		BaseProcessor:   baseProcessor,
		amazonProcessor: sharedAmazonProcessor,
	}

	// 创建 WorkerPool（内部管理）
	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)

	// 初始化任务处理器
	p.taskHandler = NewTaskHandler(p)

	// 使用统一的管道构建方法
	p.pipelineExecutor = p.buildPipelineExecutor()

	return p
}

// ProcessTask 处理TEMU任务
func (p *TemuProcessor) ProcessTask(ctx context.Context, task *model.Task) error {
	logger := p.GetLogger()
	logger.Infof("[TEMU] 开始处理任务: ID=%d, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// TEMU特定的任务处理逻辑
	if err := p.processTemuProduct(ctx, *task); err != nil {
		logger.Errorf("[TEMU] 处理产品失败: %v", err)
		return err
	}

	logger.Infof("[TEMU] 任务处理完成: ID=%d", task.ID)
	return nil
}

// processTemuProduct 处理TEMU产品
func (p *TemuProcessor) processTemuProduct(ctx context.Context, task model.Task) error {
	logger := p.GetLogger()
	logger.Infof("[TEMU] 处理产品: %s", task.ProductID)

	// 使用任务处理器处理任务
	if err := p.taskHandler.ProcessTask(ctx, task, p.pipelineExecutor); err != nil {
		return fmt.Errorf("任务处理失败: %w", err)
	}

	logger.Infof("[TEMU] 产品处理完成: %s", task.ProductID)
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

	p.GetLogger().Info("[TEMU] 任务处理器启动完成")
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
	p.GetLogger().Info("[TEMU] 关闭TEMU任务处理器")

	// 关闭基础组件
	p.CloseBase(ctx)

	// 关闭Amazon处理器
	if p.amazonProcessor != nil {
		p.amazonProcessor.Shutdown()
	}

	p.GetLogger().Info("[TEMU] 任务处理器已关闭")
}
