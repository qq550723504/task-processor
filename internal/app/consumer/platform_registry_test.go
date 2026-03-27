package consumer

import (
	"context"
	"testing"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

func TestRegisterAmazonProcessorSkipsDisabledPlatform(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	sharedResourceCalls := 0

	registry := NewPlatformRegistry(&config.Config{}, logger, "temu", PlatformRegistryDependencies{
		SharedResourceProvider: func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*SharedResources, error) {
			sharedResourceCalls++
			return &SharedResources{}, nil
		},
	})

	serviceManager := &ServiceManager{
		rabbitmqService: &RabbitMQService{
			processorRegistry: NewTaskProcessorRegistry(nil, nil, nil, nil, logger),
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
