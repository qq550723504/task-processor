package consumer

import (
	"context"
	"fmt"
	"strings"

	platformAmazon "task-processor/internal/amazon"
	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type processorRegistration struct {
	name        string
	needsAmazon bool
	register    func(context.Context, *PlatformRegistry, *ServiceManager) error
}

type PlatformRegistry struct {
	config                 *config.Config
	logger                 *logrus.Logger
	managementClient       *management.ClientManager
	sharedCrawlSource      *amazon.AmazonProcessor
	sharedProductFetcher   appfetcher.ProductFetcher
	rabbitmqClient         *rabbitmq.Client
	enabledPlatforms       []string
	processorCreators      ProcessorCreators
	sharedResourceProvider SharedResourceProvider
}

func NewPlatformRegistry(cfg *config.Config, logger *logrus.Logger, platformsStr string, deps PlatformRegistryDependencies) *PlatformRegistry {
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
		r.logger.Warn("RabbitMQ client unavailable; distributed fetching is unavailable")
	}

	registrations := r.buildProcessorRegistrations()
	if err := r.initializeSharedResources(r.anyRegistrationNeedsAmazon(registrations)); err != nil {
		return fmt.Errorf("initialize shared resources: %w", err)
	}

	for _, registration := range registrations {
		if !r.isPlatformEnabled(registration.name) {
			r.logger.Debugf("skipping %s platform registration", strings.ToUpper(registration.name))
			continue
		}
		if err := registration.register(ctx, r, serviceManager); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformRegistry) buildProcessorRegistrations() []processorRegistration {
	return []processorRegistration{
		{
			name:        "amazon",
			needsAmazon: false,
			register: func(ctx context.Context, registry *PlatformRegistry, serviceManager *ServiceManager) error {
				registry.logger.Info("registering Amazon processor")
				amazonProcessor := platformAmazon.NewProcessor(ctx, registry.config, registry.logger)
				if err := serviceManager.RegisterProcessor("amazon", amazonProcessor); err != nil {
					return fmt.Errorf("register Amazon processor: %w", err)
				}
				registry.logger.Info("Amazon processor registered")
				return nil
			},
		},
		{
			name:        "temu",
			needsAmazon: false,
			register: func(ctx context.Context, registry *PlatformRegistry, serviceManager *ServiceManager) error {
				registry.logger.Info("registering TEMU processor")
				creator := registry.processorCreators.TemuProcessorCreator
				if creator == nil {
					return fmt.Errorf("TEMU processor creator not configured")
				}
				productFetcher, err := buildPlatformProductFetcher(
					registry.config,
					"temu",
					registry.managementClient,
					registry.sharedCrawlSource,
					registry.rabbitmqClient,
				)
				if err != nil {
					return fmt.Errorf("build TEMU product fetcher: %w", err)
				}

				temuProcessor, err := creator(ctx, registry.config, registry.logger, temu.Dependencies{
					ManagementClient: registry.managementClient,
					ProductFetcher:   productFetcher,
					RabbitMQClient:   registry.rabbitmqClient,
				})
				if err != nil {
					return fmt.Errorf("create TEMU processor: %w", err)
				}
				if err := serviceManager.RegisterProcessor("temu", temuProcessor); err != nil {
					return fmt.Errorf("register TEMU processor: %w", err)
				}
				registry.logger.Info("TEMU processor registered")
				return nil
			},
		},
		{
			name:        "shein",
			needsAmazon: false,
			register: func(ctx context.Context, registry *PlatformRegistry, serviceManager *ServiceManager) error {
				registry.logger.Info("registering SHEIN processor")
				creator := registry.processorCreators.SheinProcessorCreator
				if creator == nil {
					return fmt.Errorf("SHEIN processor creator not configured")
				}
				productFetcher, err := buildPlatformProductFetcher(
					registry.config,
					"shein",
					registry.managementClient,
					registry.sharedCrawlSource,
					registry.rabbitmqClient,
				)
				if err != nil {
					return fmt.Errorf("build SHEIN product fetcher: %w", err)
				}

				sheinProcessor, err := creator(ctx, registry.config, registry.logger, pipeline.Dependencies{
					ManagementClient: registry.managementClient,
					ProductFetcher:   productFetcher,
					RabbitMQClient:   registry.rabbitmqClient,
				})
				if err != nil {
					return fmt.Errorf("create SHEIN processor: %w", err)
				}
				if err := serviceManager.RegisterProcessor("shein", sheinProcessor); err != nil {
					return fmt.Errorf("register SHEIN processor: %w", err)
				}
				registry.logger.Info("SHEIN processor registered")
				return nil
			},
		},
	}
}

func (r *PlatformRegistry) initializeSharedResources(needsAmazon bool) error {
	if r.sharedResourceProvider == nil {
		return fmt.Errorf("shared resource provider not configured")
	}
	resources, err := r.sharedResourceProvider(r.config, r.logger, needsAmazon)
	if err != nil {
		return err
	}

	r.managementClient = resources.ManagementClient
	r.sharedCrawlSource = resources.CrawlSource
	r.sharedProductFetcher = resources.ProductFetcher
	r.logger.Info("shared resources initialized")
	return nil
}

func (r *PlatformRegistry) RegisterTemuProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	return r.registerSinglePlatform(ctx, serviceManager, "temu")
}

func (r *PlatformRegistry) RegisterSheinProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	return r.registerSinglePlatform(ctx, serviceManager, "shein")
}

func (r *PlatformRegistry) RegisterAmazonProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	return r.registerSinglePlatform(ctx, serviceManager, "amazon")
}

func (r *PlatformRegistry) registerSinglePlatform(ctx context.Context, serviceManager *ServiceManager, platform string) error {
	r.rabbitmqClient = serviceManager.GetClient()

	if !r.isPlatformEnabled(platform) {
		r.logger.Debugf("skipping %s platform registration", strings.ToUpper(platform))
		return nil
	}

	for _, registration := range r.buildProcessorRegistrations() {
		if registration.name != platform {
			continue
		}
		needsAmazon := registration.needsAmazon || platformUsesLocalFetcher(r.config, platform)
		if err := r.initializeSharedResources(needsAmazon); err != nil {
			return err
		}
		return registration.register(ctx, r, serviceManager)
	}

	return fmt.Errorf("unsupported platform: %s", platform)
}

func (r *PlatformRegistry) anyRegistrationNeedsAmazon(registrations []processorRegistration) bool {
	for _, registration := range registrations {
		if r.isPlatformEnabled(registration.name) && (registration.needsAmazon || platformUsesLocalFetcher(r.config, registration.name)) {
			return true
		}
	}
	return false
}

func (r *PlatformRegistry) isPlatformEnabled(platform string) bool {
	return containsPlatform(r.enabledPlatforms, platform)
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
	return r.sharedCrawlSource
}

func (r *PlatformRegistry) GetSharedCrawlSource() *amazon.AmazonProcessor {
	return r.sharedCrawlSource
}

func (r *PlatformRegistry) GetSharedProductFetcher() appfetcher.ProductFetcher {
	return r.sharedProductFetcher
}

func (r *PlatformRegistry) GetManagementClient() *management.ClientManager {
	return r.managementClient
}
