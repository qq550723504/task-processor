package pipeline

import (
	"context"
	"fmt"

	"task-processor/internal/app/ports"
	"task-processor/internal/app/processor"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	types "task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/shein/aicache"

	"github.com/sirupsen/logrus"
)

type Dependencies struct {
	ManagementClient *management.ClientManager
	ProductSource    ports.ProductSource
	RabbitMQClient   *rabbitmq.Client
}

type SheinProcessor struct {
	*processor.BaseProcessor
	amazonProcessor ports.ProductSource
	rabbitmqClient  *rabbitmq.Client
	taskHandler     *TaskHandler
	pipeline        *Pipeline
	aiCache         *aicache.Cache
}

func NewSheinProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps Dependencies) (*SheinProcessor, error) {
	if deps.ManagementClient == nil {
		logger.Error("[SHEIN] ManagementClient is required")
		return nil, fmt.Errorf("managementClient is required")
	}
	if deps.ProductSource == nil {
		logger.Error("[SHEIN] ProductSource is required")
		return nil, fmt.Errorf("productSource is required")
	}

	if deps.RabbitMQClient != nil {
		logger.Info("[SHEIN] using RabbitMQ client for distributed fetching")
	} else {
		logger.Warn("[SHEIN] RabbitMQ client not provided, using local fetching")
	}

	baseProcessor := processor.NewBaseProcessor(ctx, &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: deps.ManagementClient,
		Logger:           logger,
		Platform:         "SHEIN",
	})

	p := &SheinProcessor{
		BaseProcessor:   baseProcessor,
		amazonProcessor: deps.ProductSource,
		rabbitmqClient:  deps.RabbitMQClient,
	}

	if cfg.Database == nil {
		logger.Warn("[SHEIN] database config is nil, AI cache will fall back to memory")
	}
	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		logger.Warnf("[SHEIN] database unavailable, AI cache falling back to memory: %v", err)
	}
	p.aiCache = aicache.New(db)

	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)
	p.taskHandler = NewTaskHandler(p)
	p.pipeline = p.buildPipeline()

	return p, nil
}

func (p *SheinProcessor) buildPipeline() *Pipeline {
	return CreateTaskProcessingPipeline(p, p.GetConfig())
}

func (p *SheinProcessor) Start(ctx context.Context) error {
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	p.GetLogger().Info("[SHEIN] processor started")
	return nil
}

func (p *SheinProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var task types.Task
	if err := jsonx.UnmarshalString(job.TaskData, &task, "parse task data"); err != nil {
		return err
	}

	return p.taskHandler.ProcessTask(ctx, task, p.pipeline)
}

func (p *SheinProcessor) GetAICache() *aicache.Cache {
	return p.aiCache
}

func (p *SheinProcessor) GetAmazonProcessor() ports.ProductSource {
	return p.amazonProcessor
}

func (p *SheinProcessor) Close(ctx context.Context) {
	p.GetLogger().Info("[SHEIN] closing processor")
	p.CloseBase(ctx)
	p.GetLogger().Info("[SHEIN] processor closed")
}
