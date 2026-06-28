package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/app/bootstrap/fetchers"
	bootstrapprocessors "task-processor/internal/app/bootstrap/processors"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/listingadmin"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func buildTemuProcessor(svc *appServices, logger *logrus.Logger) (*temu.TemuProcessor, error) {
	deps, err := buildTemuProcessorDependencies(svc)
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor dependencies: %w", err)
	}
	proc, err := bootstrapprocessors.CreateTemuProcessor(context.Background(), svc.cfg, logger, deps)
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor: %w", err)
	}
	return proc, nil
}

func buildSheinProcessor(svc *appServices, logger *logrus.Logger) (*pipeline.SheinProcessor, error) {
	deps, err := buildSheinProcessorDependencies(svc)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor dependencies: %w", err)
	}
	proc, err := bootstrapprocessors.CreateSheinProcessor(context.Background(), svc.cfg, logger, deps)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor: %w", err)
	}
	return proc, nil
}

func buildTemuProcessorDependencies(svc *appServices) (temu.Dependencies, error) {
	if svc.processorRuntime == nil {
		return temu.Dependencies{}, fmt.Errorf("TEMU processor runtime is not configured")
	}

	productFetcher, err := fetchers.BuildPlatformProductFetcher(
		svc.cfg,
		"temu",
		svc.rawJSONDataClient,
		svc.amazonCrawler,
		svc.rabbitmqClient,
	)
	if err != nil {
		return temu.Dependencies{}, err
	}

	return temu.BuildDependencies(context.Background(), svc.processorRuntime, productFetcher, svc.rabbitmqClient), nil
}

func buildSheinProcessorDependencies(svc *appServices) (pipeline.Dependencies, error) {
	if svc.processorRuntime == nil {
		return pipeline.Dependencies{}, fmt.Errorf("SHEIN processor runtime is not configured")
	}

	productFetcher, err := fetchers.BuildPlatformProductFetcher(
		svc.cfg,
		"shein",
		svc.rawJSONDataClient,
		svc.amazonCrawler,
		svc.rabbitmqClient,
	)
	if err != nil {
		return pipeline.Dependencies{}, err
	}

	return pipeline.BuildDependencies(context.Background(), sheinDependencyRuntimeAdapter{ProcessorRuntime: svc.processorRuntime}, productFetcher, svc.rabbitmqClient), nil
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
