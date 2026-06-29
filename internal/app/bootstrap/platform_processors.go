package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/app/bootstrap/fetchers"
	bootstrapprocessors "task-processor/internal/app/bootstrap/processors"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/ports"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingadmin"
	"task-processor/internal/product"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type platformProcessorResources struct {
	rawJSONDataClient product.RawJsonDataClient
	processorRuntime  consumer.ProcessorRuntime
	crawlSource       ports.CrawlSource
	rabbitmqClient    *rabbitmq.Client
}

func newPlatformProcessorResources(rawJSONDataClient product.RawJsonDataClient, processorRuntime consumer.ProcessorRuntime, crawlSource ports.CrawlSource, rabbitmqClient *rabbitmq.Client) platformProcessorResources {
	return platformProcessorResources{
		rawJSONDataClient: rawJSONDataClient,
		processorRuntime:  processorRuntime,
		crawlSource:       crawlSource,
		rabbitmqClient:    rabbitmqClient,
	}
}

func buildTemuProcessor(cfg *config.Config, resources platformProcessorResources, logger *logrus.Logger) (*temu.TemuProcessor, error) {
	deps, err := buildTemuProcessorDependencies(cfg, resources)
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor dependencies: %w", err)
	}
	proc, err := bootstrapprocessors.CreateTemuProcessor(context.Background(), cfg, logger, deps)
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor: %w", err)
	}
	return proc, nil
}

func buildSheinProcessor(cfg *config.Config, resources platformProcessorResources, logger *logrus.Logger) (*pipeline.SheinProcessor, error) {
	deps, err := buildSheinProcessorDependencies(cfg, resources)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor dependencies: %w", err)
	}
	proc, err := bootstrapprocessors.CreateSheinProcessor(context.Background(), cfg, logger, deps)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor: %w", err)
	}
	return proc, nil
}

func buildTemuProcessorDependencies(cfg *config.Config, resources platformProcessorResources) (temu.Dependencies, error) {
	if resources.processorRuntime == nil {
		return temu.Dependencies{}, fmt.Errorf("TEMU processor runtime is not configured")
	}

	productFetcher, err := fetchers.BuildPlatformProductFetcher(
		cfg,
		"temu",
		resources.rawJSONDataClient,
		resources.crawlSource,
		resources.rabbitmqClient,
	)
	if err != nil {
		return temu.Dependencies{}, err
	}

	return temu.BuildDependencies(context.Background(), resources.processorRuntime, productFetcher, resources.rabbitmqClient), nil
}

func buildSheinProcessorDependencies(cfg *config.Config, resources platformProcessorResources) (pipeline.Dependencies, error) {
	if resources.processorRuntime == nil {
		return pipeline.Dependencies{}, fmt.Errorf("SHEIN processor runtime is not configured")
	}

	productFetcher, err := fetchers.BuildPlatformProductFetcher(
		cfg,
		"shein",
		resources.rawJSONDataClient,
		resources.crawlSource,
		resources.rabbitmqClient,
	)
	if err != nil {
		return pipeline.Dependencies{}, err
	}

	return pipeline.BuildDependencies(context.Background(), sheinDependencyRuntimeAdapter{ProcessorRuntime: resources.processorRuntime}, productFetcher, resources.rabbitmqClient), nil
}

type sheinDependencyRuntimeAdapter struct {
	consumer.ProcessorRuntime
}

func (a sheinDependencyRuntimeAdapter) GetStoreAPI() listingadmin.StoreAPI {
	if a.ProcessorRuntime == nil {
		return nil
	}
	return a.ProcessorRuntime.GetStoreAPI()
}

func (a sheinDependencyRuntimeAdapter) GetImageDownloader() interface {
	DownloadImage(url string) ([]byte, error)
} {
	if a.ProcessorRuntime == nil {
		return nil
	}
	return a.ProcessorRuntime.GetImageDownloader()
}

func (a sheinDependencyRuntimeAdapter) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if a.ProcessorRuntime == nil {
		return nil, nil
	}
	status, err := a.ProcessorRuntime.GetTaskStatus(taskID)
	if err != nil || status == nil {
		return nil, err
	}
	return status, nil
}
