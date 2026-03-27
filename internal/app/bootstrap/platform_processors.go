package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func buildTemuProcessor(svc *appServices, logger *logrus.Logger) (*temu.TemuProcessor, error) {
	proc, err := createTemuProcessor(context.Background(), svc.cfg, logger, buildTemuProcessorDependencies(svc))
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor: %w", err)
	}
	return proc, nil
}

func buildSheinProcessor(svc *appServices, logger *logrus.Logger) (*pipeline.SheinProcessor, error) {
	proc, err := createSheinProcessor(context.Background(), svc.cfg, logger, buildSheinProcessorDependencies(svc))
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor: %w", err)
	}
	return proc, nil
}

func buildTemuProcessorDependencies(svc *appServices) temu.Dependencies {
	return temu.Dependencies{
		ManagementClient: svc.managementClient,
		ProductSource:    svc.amazonCrawler,
		RabbitMQClient:   svc.rabbitmqClient,
	}
}

func buildSheinProcessorDependencies(svc *appServices) pipeline.Dependencies {
	return pipeline.Dependencies{
		ManagementClient: svc.managementClient,
		ProductSource:    svc.amazonCrawler,
		RabbitMQClient:   svc.rabbitmqClient,
	}
}
