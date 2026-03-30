package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func buildTemuProcessor(svc *appServices, logger *logrus.Logger) (*temu.TemuProcessor, error) {
	deps, err := buildTemuProcessorDependencies(svc)
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor dependencies: %w", err)
	}
	proc, err := createTemuProcessor(context.Background(), svc.cfg, logger, deps)
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
	proc, err := createSheinProcessor(context.Background(), svc.cfg, logger, deps)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor: %w", err)
	}
	return proc, nil
}

func buildTemuProcessorDependencies(svc *appServices) (temu.Dependencies, error) {
	productFetcher, err := buildSharedProductFetcher(
		svc.cfg,
		svc.managementClient.GetRawJsonDataAdapter(),
		svc.amazonCrawler,
		svc.rabbitmqClient,
	)
	if err != nil {
		return temu.Dependencies{}, err
	}

	return temu.Dependencies{
		ManagementClient: svc.managementClient,
		ProductFetcher:   productFetcher,
		RabbitMQClient:   svc.rabbitmqClient,
	}, nil
}

func buildSheinProcessorDependencies(svc *appServices) (pipeline.Dependencies, error) {
	productFetcher, err := buildSharedProductFetcher(
		svc.cfg,
		svc.managementClient.GetRawJsonDataAdapter(),
		svc.amazonCrawler,
		svc.rabbitmqClient,
	)
	if err != nil {
		return pipeline.Dependencies{}, err
	}

	return pipeline.Dependencies{
		ManagementClient: svc.managementClient,
		ProductFetcher:   productFetcher,
		RabbitMQClient:   svc.rabbitmqClient,
	}, nil
}
