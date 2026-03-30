package runner

import (
	"context"
	"fmt"

	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"
)

type managedProcessor interface {
	task.PlatformProcessor
	Start(ctx context.Context) error
	Close(ctx context.Context)
}

type processorRuntimeModule struct {
	name    string
	enabled func(*config.Config) bool
	get     func(*processorServiceImpl) managedProcessor
	start   func(context.Context, *processorServiceImpl, *config.Config) error
}

func (s *processorServiceImpl) processorModules() []processorRuntimeModule {
	return []processorRuntimeModule{
		{
			name:    "temu",
			enabled: func(cfg *config.Config) bool { return cfg.Platforms.Temu.Enabled },
			get: func(s *processorServiceImpl) managedProcessor {
				if s.temuProcessor == nil {
					return nil
				}
				return s.temuProcessor
			},
			start: func(ctx context.Context, s *processorServiceImpl, cfg *config.Config) error {
				creator := s.resolveTemuProcessorCreator()
				if creator == nil {
					return fmt.Errorf("TEMU processor creator not configured")
				}
				productFetcher, err := buildRuntimeProductFetcher(cfg, s)
				if err != nil {
					return err
				}

				p, err := creator(ctx, cfg, s.logger, temu.Dependencies{
					ManagementClient: s.managementClient,
					ProductFetcher:   productFetcher,
					RabbitMQClient:   s.rabbitmqClient,
				})
				if err != nil {
					return fmt.Errorf("create TEMU processor: %w", err)
				}
				if err := p.Start(ctx); err != nil {
					return fmt.Errorf("start TEMU processor: %w", err)
				}
				s.temuProcessor = p
				s.logger.Info("TEMU processor started")
				return nil
			},
		},
		{
			name:    "shein",
			enabled: func(cfg *config.Config) bool { return cfg.Platforms.Shein.Enabled },
			get: func(s *processorServiceImpl) managedProcessor {
				if s.sheinProcessor == nil {
					return nil
				}
				return s.sheinProcessor
			},
			start: func(ctx context.Context, s *processorServiceImpl, cfg *config.Config) error {
				creator := s.resolveSheinProcessorCreator()
				if creator == nil {
					return fmt.Errorf("SHEIN processor creator not configured")
				}
				productFetcher, err := buildRuntimeProductFetcher(cfg, s)
				if err != nil {
					return err
				}

				p, err := creator(ctx, cfg, s.logger, pipeline.Dependencies{
					ManagementClient: s.managementClient,
					ProductFetcher:   productFetcher,
					RabbitMQClient:   s.rabbitmqClient,
				})
				if err != nil {
					return fmt.Errorf("create SHEIN processor: %w", err)
				}
				if err := p.Start(ctx); err != nil {
					return fmt.Errorf("start SHEIN processor: %w", err)
				}
				s.sheinProcessor = p
				s.logger.Info("SHEIN processor started")
				return nil
			},
		},
	}
}

func buildRuntimeProductFetcher(cfg *config.Config, s *processorServiceImpl) (appfetcher.ProductFetcher, error) {
	if s.managementClient == nil {
		return nil, fmt.Errorf("management client not initialized")
	}

	factory := appfetcher.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(
		cfg,
		s.managementClient.GetRawJsonDataAdapter(),
		s.crawlSource,
		s.rabbitmqClient,
	)
	if err != nil {
		return nil, fmt.Errorf("create distributed product fetcher: %w", err)
	}

	return fetcher, nil
}
