package shein

import (
	"context"
	"fmt"

	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
)

type Module struct{}

func NewModule() Module {
	return Module{}
}

func (Module) Name() string {
	return "shein"
}

func (Module) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Platforms.Shein.Enabled
}

func (m Module) NeedsAmazon(cfg *config.Config) bool {
	return consumer.PlatformUsesLocalFetcher(cfg, m.Name())
}

func (m Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
	productFetcher, err := consumer.BuildPlatformProductFetcher(
		rt.Config,
		m.Name(),
		rt.ManagementClient,
		rt.CrawlSource,
		rt.RabbitMQClient,
	)
	if err != nil {
		return fmt.Errorf("build SHEIN product fetcher: %w", err)
	}

	processor, err := pipeline.NewSheinProcessor(ctx, rt.Config, rt.Logger, pipeline.Dependencies{
		ManagementClient: rt.ManagementClient,
		ProductFetcher:   productFetcher,
		RabbitMQClient:   rt.RabbitMQClient,
	})
	if err != nil {
		return fmt.Errorf("create SHEIN processor: %w", err)
	}
	if err := registry.RegisterProcessor(m.Name(), processor); err != nil {
		return fmt.Errorf("register SHEIN processor: %w", err)
	}
	return nil
}
