package temu

import (
	"context"
	"fmt"

	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/platformbase"
	"task-processor/internal/prompt"
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
	return platformbase.PlatformUsesLocalFetcher(cfg, m.Name())
}

func (m Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
	productReader := rt.ProductReader()
	productCache := rt.ProductCache()
	productFetcherStats := rt.ProductFetcherStats()
	if productReader == nil || productCache == nil || productFetcherStats == nil {
		return fmt.Errorf("TEMU product reader, cache, and stats are not configured")
	}

	processor, err := temuprocessor.NewTemuProcessor(ctx, rt.Config(), rt.Logger(), temuprocessor.BuildDependencies(ctx, rt.ProcessorRuntime(), productReader, productCache, productFetcherStats, rt.RabbitMQClient()))
	if err != nil {
		return fmt.Errorf("create TEMU processor: %w", err)
	}
	if err := registry.RegisterProcessor(m.Name(), processor); err != nil {
		return fmt.Errorf("register TEMU processor: %w", err)
	}
	return nil
}

func (Module) ConfigureListingRuntime(ctx context.Context, rt consumer.PlatformRuntimeContext) error {
	if err := initPrompts(ctx, rt); err != nil {
		rt.Logger().Warnf("prompt init failed, fallback will be used: %v", err)
	}
	return consumer.EnableDynamicStoreAssignment(rt.Config(), rt.Logger(), rt.StoreAssignmentRuntime())
}

func initPrompts(ctx context.Context, rt consumer.PlatformRuntimeContext) error {
	cfg := rt.Config()
	if cfg == nil {
		return nil
	}
	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	return prompt.InitGlobal(ctx, promptsDir, cfg.Prompts.HotReload, rt.Logger().WithField("component", "prompt"))
}
