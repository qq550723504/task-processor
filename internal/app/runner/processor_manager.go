package runner

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"
)

func (s *processorServiceImpl) startProcessors(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("starting processors")

	if cfg.Platforms.Temu.Enabled {
		if err := s.startTemuProcessor(ctx, cfg); err != nil {
			return fmt.Errorf("start TEMU processor: %w", err)
		}
	} else {
		s.logger.Info("TEMU processor disabled, skipping")
	}

	if cfg.Platforms.Shein.Enabled {
		if err := s.startSheinProcessor(ctx, cfg); err != nil {
			return fmt.Errorf("start SHEIN processor: %w", err)
		}
	} else {
		s.logger.Info("SHEIN processor disabled, skipping")
	}

	if !cfg.Platforms.Temu.Enabled && !cfg.Platforms.Shein.Enabled {
		s.logger.Warn("all processors disabled, system will run idle")
	}

	s.logger.Info("processor startup flow complete")
	return nil
}

func (s *processorServiceImpl) startTemuProcessor(ctx context.Context, cfg *config.Config) error {
	if s.amazonProcessor == nil {
		return fmt.Errorf("Amazon processor not initialized")
	}
	creator := s.resolveTemuProcessorCreator()
	if creator == nil {
		return fmt.Errorf("TEMU processor creator not configured")
	}

	p, err := creator(ctx, cfg, s.logger, temu.Dependencies{
		ManagementClient: s.managementClient,
		ProductSource:    s.amazonProcessor,
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
}

func (s *processorServiceImpl) startSheinProcessor(ctx context.Context, cfg *config.Config) error {
	if s.amazonProcessor == nil {
		return fmt.Errorf("Amazon processor not initialized")
	}
	creator := s.resolveSheinProcessorCreator()
	if creator == nil {
		return fmt.Errorf("SHEIN processor creator not configured")
	}

	p, err := creator(ctx, cfg, s.logger, pipeline.Dependencies{
		ManagementClient: s.managementClient,
		ProductSource:    s.amazonProcessor,
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
}

func (s *processorServiceImpl) stopAllProcessors(ctx context.Context) {
	if s.temuProcessor != nil {
		s.temuProcessor.Close(ctx)
		s.logger.Info("TEMU processor stopped")
	}

	if s.sheinProcessor != nil {
		s.sheinProcessor.Close(ctx)
		s.logger.Info("SHEIN processor stopped")
	}

	s.logger.Info("all processors stopped")
}
