package runner

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/task"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
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
				if s.processorRuntime == nil {
					return fmt.Errorf("TEMU processor runtime not configured")
				}
				productFetcher, err := buildRuntimeProductFetcher(cfg, s, "temu")
				if err != nil {
					return err
				}
				p, err := creator(ctx, cfg, s.logger, temu.BuildDependencies(ctx, s.processorRuntime, productFetcher, s.rabbitmqClient))
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
				if s.processorRuntime == nil {
					return fmt.Errorf("SHEIN processor runtime not configured")
				}
				productFetcher, err := buildRuntimeProductFetcher(cfg, s, "shein")
				if err != nil {
					return err
				}
				p, err := creator(ctx, cfg, s.logger, pipeline.BuildDependencies(ctx, s.processorRuntime, productFetcher, s.rabbitmqClient))
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

func buildRuntimeProductFetcher(cfg *config.Config, s *processorServiceImpl, platform string) (appfetcher.ProductFetcher, error) {
	if s.rawJSONDataClient == nil {
		return nil, fmt.Errorf("raw json data client not initialized")
	}

	factory := appfetcher.NewFetcherFactory()
	fetcherType, err := resolveRuntimePlatformFetcherType(cfg, platform)
	if err != nil {
		return nil, fmt.Errorf("resolve %s fetch mode: %w", platform, err)
	}

	if fetcherType == "" {
		fetcher, err := factory.CreateFetcherFromConfig(
			cfg,
			s.rawJSONDataClient,
			s.crawlSource,
			s.rabbitmqClient,
		)
		if err != nil {
			return nil, fmt.Errorf("create %s product fetcher: %w", platform, err)
		}
		return fetcher, nil
	}

	fetcher, err := factory.CreateFetcher(
		fetcherType,
		s.rawJSONDataClient,
		&cfg.Amazon,
		s.crawlSource,
		s.rabbitmqClient,
	)
	if err != nil {
		return nil, fmt.Errorf("create %s product fetcher: %w", platform, err)
	}
	return fetcher, nil
}

func resolveRuntimePlatformFetcherType(cfg *config.Config, platform string) (appfetcher.FetcherType, error) {
	if cfg == nil {
		return "", nil
	}

	mode := "auto"
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "temu":
		mode = strings.TrimSpace(cfg.Platforms.Temu.FetchMode)
	case "shein":
		mode = strings.TrimSpace(cfg.Platforms.Shein.FetchMode)
	}

	switch strings.ToLower(mode) {
	case "", "auto":
		return "", nil
	case "local":
		return appfetcher.LocalFetcher, nil
	case "distributed":
		return appfetcher.DistributedFetcher, nil
	case "remote-api", "remoteapi", "remote_api":
		return appfetcher.RemoteAPIFetcher, nil
	default:
		return "", fmt.Errorf("unsupported fetch mode %q", mode)
	}
}
