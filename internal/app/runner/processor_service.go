package runner

import (
	"context"

	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type amazonCrawler interface {
	Process(url string, zipcode string) (*model.Product, error)
	ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error)
	Shutdown()
}

type ProcessorService interface {
	StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error
	StopProcessors() error
	GetStatus() map[string]any
}

func NewProcessorServiceWithCreators(
	logger *logrus.Logger,
	managementClient *management.ClientManager,
	amazonProcessor amazonCrawler,
	deps ProcessorDependencies,
) ProcessorService {
	deps = normalizeProcessorDependencies(deps)

	return &processorServiceImpl{
		logger:                logger,
		lifecycleManager:      lifecycle.NewLifecycleManager(logger),
		managementClient:      managementClient,
		amazonProcessor:       amazonProcessor,
		temuProcessorCreator:  deps.TemuProcessorCreator,
		sheinProcessorCreator: deps.SheinProcessorCreator,
	}
}

func normalizeProcessorDependencies(deps ProcessorDependencies) ProcessorDependencies {
	defaultDeps := buildProcessorDependencies()
	if deps.TemuProcessorCreator == nil {
		deps.TemuProcessorCreator = defaultDeps.TemuProcessorCreator
	}
	if deps.SheinProcessorCreator == nil {
		deps.SheinProcessorCreator = defaultDeps.SheinProcessorCreator
	}

	return deps
}
