package consumer

import (
	"context"
	"fmt"
	"strings"

	platformAmazon "task-processor/internal/amazon"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type PlatformRegistry struct {
	config                 *config.Config
	logger                 *logrus.Logger
	managementClient       *management.ClientManager
	sharedAmazonProcessor  *amazon.AmazonProcessor
	rabbitmqClient         *rabbitmq.Client
	enabledPlatforms       []string
	processorCreators      ProcessorCreators
	sharedResourceProvider SharedResourceProvider
}

func NewPlatformRegistry(cfg *config.Config, logger *logrus.Logger, platformsStr string) *PlatformRegistry {
	return NewPlatformRegistryWithDependencies(cfg, logger, platformsStr, PlatformRegistryDependencies{
		ProcessorCreators: defaultProcessorCreators(),
	})
}

func NewPlatformRegistryWithDependencies(cfg *config.Config, logger *logrus.Logger, platformsStr string, deps PlatformRegistryDependencies) *PlatformRegistry {
	var enabledPlatforms []string
	if platformsStr != "" {
		enabledPlatforms = parsePlatformList(platformsStr)
	} else {
		enabledPlatforms = getEnabledPlatformsFromConfig(cfg)
	}

	logger.Infof("enabled platforms: %v", enabledPlatforms)

	return &PlatformRegistry{
		config:                 cfg,
		logger:                 logger,
		enabledPlatforms:       enabledPlatforms,
		processorCreators:      deps.ProcessorCreators,
		sharedResourceProvider: deps.SharedResourceProvider,
	}
}

func (r *PlatformRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager) error {
	r.logger.Info("registering platform processors")

	r.rabbitmqClient = serviceManager.GetClient()
	if r.rabbitmqClient != nil {
		r.logger.Info("RabbitMQ client available for distributed fetching")
	} else {
		r.logger.Warn("RabbitMQ client unavailable, local fetching will be used")
	}

	if err := r.initializeSharedResources(); err != nil {
		return fmt.Errorf("initialize shared resources: %w", err)
	}

	if err := r.registerAmazonPlatform(ctx, serviceManager); err != nil {
		return err
	}
	if err := r.registerTemuPlatform(ctx, serviceManager); err != nil {
		return err
	}
	if err := r.registerSheinPlatform(ctx, serviceManager); err != nil {
		return err
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformRegistry) initializeSharedResources() error {
	if r.sharedResourceProvider == nil {
		return fmt.Errorf("shared resource provider not configured")
	}
	resources, err := r.sharedResourceProvider(r.config, r.logger, r.needsAmazonProcessor())
	if err != nil {
		return err
	}

	r.managementClient = resources.ManagementClient
	r.sharedAmazonProcessor = resources.AmazonProcessor
	r.logger.Info("shared resources initialized")
	return nil
}

func (r *PlatformRegistry) needsAmazonProcessor() bool {
	return containsPlatform(r.enabledPlatforms, "temu") || containsPlatform(r.enabledPlatforms, "shein")
}

func (r *PlatformRegistry) registerAmazonPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "amazon") {
		r.logger.Debug("skipping Amazon platform registration")
		return nil
	}

	r.logger.Info("registering Amazon processor")
	amazonProcessor := platformAmazon.NewProcessor(ctx, r.config, r.logger)
	if err := serviceManager.RegisterProcessor("amazon", amazonProcessor); err != nil {
		return fmt.Errorf("register Amazon processor: %w", err)
	}

	r.logger.Info("Amazon processor registered")
	return nil
}

func (r *PlatformRegistry) registerTemuPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "temu") {
		r.logger.Debug("skipping TEMU platform registration")
		return nil
	}

	r.logger.Info("registering TEMU processor")
	creator := r.processorCreators.TemuProcessorCreator
	if creator == nil {
		return fmt.Errorf("TEMU processor creator not configured")
	}
	temuProcessor, err := creator(ctx, r.config, r.logger, temu.Dependencies{
		ManagementClient: r.managementClient,
		ProductSource:    r.sharedAmazonProcessor,
		RabbitMQClient:   r.rabbitmqClient,
	})
	if err != nil {
		return fmt.Errorf("create TEMU processor: %w", err)
	}

	if err := serviceManager.RegisterProcessor("temu", temuProcessor); err != nil {
		return fmt.Errorf("register TEMU processor: %w", err)
	}

	r.logger.Info("TEMU processor registered")
	return nil
}

func (r *PlatformRegistry) registerSheinPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "shein") {
		r.logger.Debug("skipping SHEIN platform registration")
		return nil
	}

	r.logger.Info("registering SHEIN processor")
	creator := r.processorCreators.SheinProcessorCreator
	if creator == nil {
		return fmt.Errorf("SHEIN processor creator not configured")
	}
	sheinProcessor, err := creator(ctx, r.config, r.logger, pipeline.Dependencies{
		ManagementClient: r.managementClient,
		ProductSource:    r.sharedAmazonProcessor,
		RabbitMQClient:   r.rabbitmqClient,
	})
	if err != nil {
		return fmt.Errorf("create SHEIN processor: %w", err)
	}

	if err := serviceManager.RegisterProcessor("shein", sheinProcessor); err != nil {
		return fmt.Errorf("register SHEIN processor: %w", err)
	}

	r.logger.Info("SHEIN processor registered")
	return nil
}

func (r *PlatformRegistry) RegisterTemuProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	if err := r.initForSinglePlatform(serviceManager, true); err != nil {
		return err
	}
	return r.registerTemuPlatform(ctx, serviceManager)
}

func (r *PlatformRegistry) RegisterSheinProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	if err := r.initForSinglePlatform(serviceManager, true); err != nil {
		return err
	}
	return r.registerSheinPlatform(ctx, serviceManager)
}

func (r *PlatformRegistry) RegisterAmazonProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	if err := r.initForSinglePlatform(serviceManager, false); err != nil {
		return err
	}
	return r.registerAmazonPlatform(ctx, serviceManager)
}

func (r *PlatformRegistry) initForSinglePlatform(serviceManager *ServiceManager, needsAmazon bool) error {
	r.rabbitmqClient = serviceManager.GetClient()
	if r.sharedResourceProvider == nil {
		return fmt.Errorf("shared resource provider not configured")
	}
	resources, err := r.sharedResourceProvider(r.config, r.logger, needsAmazon)
	if err != nil {
		return err
	}

	r.managementClient = resources.ManagementClient
	r.sharedAmazonProcessor = resources.AmazonProcessor
	return nil
}

func parsePlatformList(platformsStr string) []string {
	platforms := strings.Split(platformsStr, ",")
	result := make([]string, 0, len(platforms))

	for _, p := range platforms {
		trimmed := strings.TrimSpace(strings.ToLower(p))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func getEnabledPlatformsFromConfig(cfg *config.Config) []string {
	platforms := make([]string, 0)

	if cfg.Amazon.Enabled {
		platforms = append(platforms, "amazon")
	}
	if cfg.Platforms.Temu.Enabled {
		platforms = append(platforms, "temu")
	}
	if cfg.Platforms.Shein.Enabled {
		platforms = append(platforms, "shein")
	}

	return platforms
}

func containsPlatform(platforms []string, platform string) bool {
	platform = strings.ToLower(platform)
	for _, p := range platforms {
		if strings.ToLower(p) == platform {
			return true
		}
	}
	return false
}

func (r *PlatformRegistry) GetSharedAmazonProcessor() *amazon.AmazonProcessor {
	return r.sharedAmazonProcessor
}

func (r *PlatformRegistry) GetManagementClient() *management.ClientManager {
	return r.managementClient
}
