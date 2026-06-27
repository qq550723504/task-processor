package temu

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/processor"
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"
	temuclient "task-processor/internal/temu/api/client"

	"github.com/sirupsen/logrus"
)

type processorRuntime interface {
	temuclient.StoreRuntime
	taskstatus.RuntimeTaskStatusUpdater
	GetFilterRuleClient() listingadmin.FilterRuleAPI
	GetProductImportMappingClient() listingadmin.ProductImportMappingAPI
	GetProfitRuleClient() listingadmin.ProfitRuleAPI
}

type Dependencies struct {
	ProcessorRuntime  processorRuntime
	TaskStatusRuntime taskstatus.RuntimeTaskStatusUpdater
	MemoryManager     *state.MemoryManager
	ProductFetcher    appfetcher.ProductFetcher
	RabbitMQClient    *rabbitmq.Client
}

type TemuProcessor struct {
	*processor.BaseProcessor
	processorRuntime  processorRuntime
	taskStatusRuntime taskstatus.RuntimeTaskStatusUpdater
	productFetcher    appfetcher.ProductFetcher
	rabbitmqClient    *rabbitmq.Client
	taskHandler       *TaskHandler
	pipelineExecutor  *TemuPipelineExecutor
}

func NewTemuProcessor(ctx context.Context, cfg *config.Config, loggerInstance *logrus.Logger, deps Dependencies) (*TemuProcessor, error) {
	log := logger.GetGlobalLogger("temu_processor").WithField(logger.FieldPlatform, "temu")

	if deps.ProcessorRuntime == nil {
		log.Error("ProcessorRuntime is required")
		return nil, fmt.Errorf("processorRuntime is required")
	}
	if deps.TaskStatusRuntime == nil {
		log.Error("TaskStatusRuntime is required")
		return nil, fmt.Errorf("taskStatusRuntime is required")
	}
	if deps.MemoryManager == nil {
		log.Error("MemoryManager is required")
		return nil, fmt.Errorf("memoryManager is required")
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

	_ = ctx
	baseProcessor := processor.NewBaseProcessorWithMemoryManager(&processor.BaseProcessorConfig{
		Config:   cfg,
		Logger:   loggerInstance,
		Platform: "TEMU",
	}, deps.MemoryManager)

	p := &TemuProcessor{
		BaseProcessor:     baseProcessor,
		processorRuntime:  deps.ProcessorRuntime,
		taskStatusRuntime: deps.TaskStatusRuntime,
		productFetcher:    deps.ProductFetcher,
		rabbitmqClient:    deps.RabbitMQClient,
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

func (p *TemuProcessor) GetTaskStatusRuntime() taskstatus.RuntimeTaskStatusUpdater {
	if p == nil {
		return nil
	}
	return p.taskStatusRuntime
}

func (p *TemuProcessor) GetStoreRuntime() temuclient.StoreRuntime {
	if p == nil {
		return nil
	}
	return p.processorRuntime
}

func (p *TemuProcessor) GetStoreClient() listingadmin.StoreAPI {
	if p == nil || p.processorRuntime == nil {
		return nil
	}
	return p.processorRuntime.GetStoreAPI()
}

func (p *TemuProcessor) GetFilterRuleClient() listingadmin.FilterRuleAPI {
	if p == nil || p.processorRuntime == nil {
		return nil
	}
	return p.processorRuntime.GetFilterRuleClient()
}

func (p *TemuProcessor) GetProductImportMappingClient() listingadmin.ProductImportMappingAPI {
	if p == nil || p.processorRuntime == nil {
		return nil
	}
	return p.processorRuntime.GetProductImportMappingClient()
}

func (p *TemuProcessor) GetProfitRuleClient() listingadmin.ProfitRuleAPI {
	if p == nil || p.processorRuntime == nil {
		return nil
	}
	return p.processorRuntime.GetProfitRuleClient()
}

func (p *TemuProcessor) Close(ctx context.Context) {
	log := p.GetLogger()
	log.Info("closing TEMU processor")

	p.CloseBase(ctx)

	log.Info("processor closed")
}
