package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func buildTemuProcessor(svc *appServices, logger *logrus.Logger) (*temu.TemuProcessor, error) {
	proc, err := temu.NewTemuProcessor(
		context.Background(),
		svc.cfg,
		logger,
		temu.Dependencies{
			ManagementClient: svc.managementClient,
			ProductSource:    svc.amazonCrawler,
			RabbitMQClient:   nil,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("build TEMU processor: %w", err)
	}
	return proc, nil
}

func buildSheinProcessor(svc *appServices, logger *logrus.Logger) (*pipeline.SheinProcessor, error) {
	proc, err := pipeline.NewSheinProcessor(
		context.Background(),
		svc.cfg,
		logger,
		pipeline.Dependencies{
			ManagementClient: svc.managementClient,
			ProductSource:    svc.amazonCrawler,
			RabbitMQClient:   nil,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN processor: %w", err)
	}
	return proc, nil
}
