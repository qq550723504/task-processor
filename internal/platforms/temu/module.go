package temu

import (
	"context"
	"fmt"

	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	temuprocessor "task-processor/internal/temu"
)

type Module struct{}

func NewModule() Module {
	return Module{}
}

func (Module) Name() string {
	return "temu"
}

func (Module) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Platforms.Temu.Enabled
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
		return fmt.Errorf("build TEMU product fetcher: %w", err)
	}

	processor, err := temuprocessor.NewTemuProcessor(ctx, rt.Config, rt.Logger, temuprocessor.Dependencies{
		ManagementClient: rt.ManagementClient,
		ProductFetcher:   productFetcher,
		RabbitMQClient:   rt.RabbitMQClient,
	})
	if err != nil {
		return fmt.Errorf("create TEMU processor: %w", err)
	}
	if err := registry.RegisterProcessor(m.Name(), processor); err != nil {
		return fmt.Errorf("register TEMU processor: %w", err)
	}
	return nil
}
