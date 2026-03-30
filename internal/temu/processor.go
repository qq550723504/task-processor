package temu

import (
	"context"
	"fmt"

	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/processor"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

type Dependencies struct {
	ManagementClient *management.ClientManager
	ProductFetcher   appfetcher.ProductFetcher
	RabbitMQClient   *rabbitmq.Client
}

type TemuProcessor struct {
	*processor.BaseProcessor
	productFetcher   appfetcher.ProductFetcher
	rabbitmqClient   *rabbitmq.Client
	taskHandler      *TaskHandler
	pipelineExecutor *TemuPipelineExecutor
}

func NewTemuProcessor(ctx context.Context, cfg *config.Config, loggerInstance *logrus.Logger, deps Dependencies) (*TemuProcessor, error) {
	log := logger.GetGlobalLogger("temu_processor").WithField(logger.FieldPlatform, "temu")

	if deps.ManagementClient == nil {
		log.Error("ManagementClient is required")
		return nil, fmt.Errorf("managementClient is required")
	}
	if deps.ProductFetcher == nil {
		log.Error("ProductFetcher is required")
		return nil, fmt.Errorf("productFetcher is required")
	}

	if deps.RabbitMQClient != nil {
		log.Info("using RabbitMQ client for distributed fetching")
	} else {
		log.Warn("RabbitMQ client not provided; distributed fetching is unavailable")
	}

	baseProcessor := processor.NewBaseProcessor(ctx, &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: deps.ManagementClient,
		Logger:           loggerInstance,
		Platform:         "TEMU",
	})

	p := &TemuProcessor{
		BaseProcessor:  baseProcessor,
		productFetcher: deps.ProductFetcher,
		rabbitmqClient: deps.RabbitMQClient,
	}

	workerPool := worker.NewPool(p, cfg.Worker)
	p.SetWorkerPool(workerPool)
	p.taskHandler = NewTaskHandler(p)
	p.pipelineExecutor = p.buildPipelineExecutor()

	return p, nil
}

func (p *TemuProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var task model.Task
	if err := jsonx.UnmarshalString(job.TaskData, &task, "parse task data"); err != nil {
		return err
	}

	log := p.GetLogger()
	log.WithFields(logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
		logger.FieldStoreID:   task.StoreID,
	}).Info("start task")

	if err := p.processTemuProduct(ctx, task); err != nil {
		log.WithError(err).WithField(logger.FieldTaskID, task.ID).Error("process product failed")
		return err
	}

	log.WithField(logger.FieldTaskID, task.ID).Info("task complete")
	return nil
}

func (p *TemuProcessor) processTemuProduct(ctx context.Context, task model.Task) error {
	log := p.GetLogger()
	log.WithField(logger.FieldProductID, task.ProductID).Info("processing product")

	if err := p.taskHandler.ProcessTask(ctx, task, p.pipelineExecutor); err != nil {
		return fmt.Errorf("task processing failed: %w", err)
	}

	log.WithField(logger.FieldProductID, task.ProductID).Info("product processed")
	return nil
}

func (p *TemuProcessor) buildPipelineExecutor() *TemuPipelineExecutor {
	builder := NewPipelineBuilder(p)
	return builder.BuildPipeline()
}

func (p *TemuProcessor) Start(ctx context.Context) error {
	if err := p.StartBase(ctx); err != nil {
		return err
	}

	p.GetLogger().Info("processor started")
	return nil
}

func (p *TemuProcessor) GetProductFetcher() appfetcher.ProductFetcher {
	return p.productFetcher
}

func (p *TemuProcessor) Close(ctx context.Context) {
	log := p.GetLogger()
	log.Info("closing TEMU processor")

	p.CloseBase(ctx)

	log.Info("processor closed")
}
