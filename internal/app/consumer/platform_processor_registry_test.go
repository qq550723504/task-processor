package consumer

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

func TestRegisterAmazonProcessorSkipsDisabledPlatform(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	sharedResourceCalls := 0

	registry := NewPlatformProcessorRegistry(&config.Config{}, logger, "temu", PlatformProcessorRegistryDependencies{
		PlatformModules: []PlatformModule{stubPlatformModule{name: "amazon"}},
		SharedResourceProvider: func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*SharedResources, error) {
			sharedResourceCalls++
			return &SharedResources{}, nil
		},
	})

	serviceManager := &ServiceManager{
		rabbitmqService: &RabbitMQService{
			processorRegistry: NewTaskProcessorRegistry(nil, nil, nil, false, nil, logger),
			logger:            logger,
		},
	}

	if err := registry.RegisterAmazonProcessor(context.Background(), serviceManager); err != nil {
		t.Fatalf("RegisterAmazonProcessor returned error: %v", err)
	}

	if sharedResourceCalls != 0 {
		t.Fatalf("expected shared resources to stay uninitialized, got %d calls", sharedResourceCalls)
	}

	if got := len(serviceManager.rabbitmqService.processorRegistry.processors); got != 0 {
		t.Fatalf("expected no processors to be registered, got %d", got)
	}
}

func TestRegisterAmazonProcessorRegistersEnabledPlatform(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	cfg := &config.Config{}
	cfg.Amazon.Enabled = true

	registry := NewPlatformProcessorRegistry(cfg, logger, "amazon", PlatformProcessorRegistryDependencies{
		PlatformModules: []PlatformModule{stubPlatformModule{name: "amazon"}},
		SharedResourceProvider: func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*SharedResources, error) {
			return &SharedResources{}, nil
		},
	})

	serviceManager := &ServiceManager{
		rabbitmqService: &RabbitMQService{
			processorRegistry: NewTaskProcessorRegistry(nil, nil, nil, false, nil, logger),
			logger:            logger,
		},
	}

	if err := registry.RegisterAmazonProcessor(context.Background(), serviceManager); err != nil {
		t.Fatalf("RegisterAmazonProcessor returned error: %v", err)
	}

	if got := len(serviceManager.rabbitmqService.processorRegistry.processors); got != 1 {
		t.Fatalf("expected one processor to be registered, got %d", got)
	}

	if _, exists := serviceManager.rabbitmqService.processorRegistry.processors["amazon"]; !exists {
		t.Fatal("expected amazon processor to be registered")
	}
}

type stubPlatformModule struct {
	name string
}

func (m stubPlatformModule) Name() string {
	return m.name
}

func (m stubPlatformModule) Enabled(cfg *config.Config) bool {
	return true
}

func (m stubPlatformModule) NeedsAmazon(cfg *config.Config) bool {
	return false
}

func (m stubPlatformModule) RegisterConsumer(ctx context.Context, rt PlatformRuntimeContext, registry ProcessorRegistrar) error {
	return registry.RegisterProcessor(m.name, registryStubProcessor{})
}

type registryStubProcessor struct{}

func (registryStubProcessor) Start(ctx context.Context) error {
	return nil
}

func (registryStubProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	return nil
}

func (registryStubProcessor) Close(ctx context.Context) {}
