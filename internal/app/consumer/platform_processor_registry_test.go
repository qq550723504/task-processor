package consumer

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

func TestRegisterPlatformsSkipsDisabledPlatform(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	registry := NewPlatformProcessorRegistry(&config.Config{}, logger, "temu", PlatformProcessorRegistryDependencies{
		PlatformModules: []PlatformModule{stubPlatformModule{name: "amazon"}},
	})

	serviceManager := newRegistryTestServiceManager(logger)

	err := registry.RegisterPlatforms(context.Background(), serviceManager, &SharedResources{}, "amazon")
	if err == nil {
		t.Fatal("expected disabled platform registration to fail")
	}
}

func TestRegisterPlatformsRegistersEnabledPlatform(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	cfg := &config.Config{}
	cfg.Amazon.Enabled = true

	registry := NewPlatformProcessorRegistry(cfg, logger, "amazon", PlatformProcessorRegistryDependencies{
		PlatformModules: []PlatformModule{stubPlatformModule{name: "amazon"}},
	})

	serviceManager := newRegistryTestServiceManager(logger)

	if err := registry.RegisterPlatforms(context.Background(), serviceManager, &SharedResources{}, "amazon"); err != nil {
		t.Fatalf("RegisterPlatforms returned error: %v", err)
	}

	if got := len(serviceManager.rabbitmqService.processorRegistry.processors); got != 1 {
		t.Fatalf("expected one processor to be registered, got %d", got)
	}

	if _, exists := serviceManager.rabbitmqService.processorRegistry.processors["amazon"]; !exists {
		t.Fatal("expected amazon processor to be registered")
	}
}

func TestSharedResourceNeedsIncludesAmazonCrawlerForLocalFetcher(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	cfg := &config.Config{}
	cfg.Platforms.Shein.Enabled = true
	cfg.Platforms.Shein.FetchMode = "local"

	registry := NewPlatformProcessorRegistry(cfg, logger, "shein", PlatformProcessorRegistryDependencies{
		PlatformModules: []PlatformModule{stubPlatformModule{name: "shein"}},
	})

	needs, err := registry.SharedResourceNeeds("shein")
	if err != nil {
		t.Fatalf("SharedResourceNeeds returned error: %v", err)
	}
	if !needs.NeedAmazonCrawler {
		t.Fatal("expected local fetcher platform to request Amazon crawler resources")
	}
}

func newRegistryTestServiceManager(logger *logrus.Logger) *ServiceManager {
	return &ServiceManager{
		rabbitmqService: &RabbitMQService{
			processorRegistry: NewTaskProcessorRegistry(nil, nil, nil, false, nil, logger),
			logger:            logger,
		},
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

func (m stubPlatformModule) ConfigureListingRuntime(ctx context.Context, rt PlatformRuntimeContext) error {
	return nil
}

type registryStubProcessor struct{}

func (registryStubProcessor) Start(ctx context.Context) error {
	return nil
}

func (registryStubProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	return nil
}

func (registryStubProcessor) Close(ctx context.Context) {}
