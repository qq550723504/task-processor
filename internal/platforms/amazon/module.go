package amazon

import (
	"context"
	"fmt"

	amazonprocessor "task-processor/internal/amazon"
	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
)

type Module struct{}

func NewModule() Module {
	return Module{}
}

func (Module) Name() string {
	return "amazon"
}

func (Module) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Amazon.Enabled
}

func (Module) NeedsAmazon(cfg *config.Config) bool {
	return false
}

func (m Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
	processor := amazonprocessor.NewProcessor(ctx, rt.Config, rt.Logger)
	if err := registry.RegisterProcessor(m.Name(), processor); err != nil {
		return fmt.Errorf("register Amazon processor: %w", err)
	}
	return nil
}

func (Module) ConfigureListingRuntime(_ context.Context, rt consumer.PlatformRuntimeContext) error {
	return consumer.EnableDynamicStoreAssignment(rt.Config, rt.Logger, rt.ServiceManager)
}
