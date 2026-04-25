package consumer

import (
	"context"
	"fmt"
	"strings"

	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

type processorRegistration struct {
	name        string
	needsAmazon bool
	register    func(context.Context, *PlatformProcessorRegistry, *ServiceManager) error
}

type PlatformProcessorRegistry struct {
	config                 *config.Config
	logger                 *logrus.Logger
	managementClient       *management.ClientManager
	sharedCrawlSource      *amazon.AmazonProcessor
	sharedProductFetcher   appfetcher.ProductFetcher
	rabbitmqClient         *rabbitmq.Client
	enabledPlatforms       []string
	sharedResourceProvider SharedResourceProvider
	platformModules        []PlatformModule
}

func NewPlatformProcessorRegistry(cfg *config.Config, logger *logrus.Logger, platformsStr string, deps PlatformProcessorRegistryDependencies) *PlatformProcessorRegistry {
	var enabledPlatforms []string
	if platformsStr != "" {
		enabledPlatforms = parsePlatformList(platformsStr)
	} else {
		enabledPlatforms = getEnabledPlatformsFromModules(cfg, deps.PlatformModules)
	}

	logger.Infof("enabled platforms: %v", enabledPlatforms)

	return &PlatformProcessorRegistry{
		config:                 cfg,
		logger:                 logger,
		enabledPlatforms:       enabledPlatforms,
		sharedResourceProvider: deps.SharedResourceProvider,
		platformModules:        deps.PlatformModules,
	}
}

func (r *PlatformProcessorRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager) error {
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

func (r *PlatformProcessorRegistry) buildProcessorRegistrations() []processorRegistration {
	registrations := make([]processorRegistration, 0, len(r.platformModules))
	for _, module := range r.platformModules {
		module := module
		registrations = append(registrations, processorRegistration{
			name:        module.Name(),
			needsAmazon: module.NeedsAmazon(r.config),
			register: func(ctx context.Context, registry *PlatformProcessorRegistry, serviceManager *ServiceManager) error {
				registry.logger.Infof("registering %s processor", strings.ToUpper(module.Name()))
				if err := module.RegisterConsumer(ctx, registry.runtimeContext(), serviceManager); err != nil {
					return err
				}
				registry.logger.Infof("%s processor registered", strings.ToUpper(module.Name()))
				return nil
			},
		})
	}
	return registrations
}

func (r *PlatformProcessorRegistry) initializeSharedResources(needsAmazon bool) error {
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

func (r *PlatformProcessorRegistry) RegisterTemuProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	return r.registerSinglePlatform(ctx, serviceManager, "temu")
}

func (r *PlatformProcessorRegistry) RegisterSheinProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	return r.registerSinglePlatform(ctx, serviceManager, "shein")
}

func (r *PlatformProcessorRegistry) RegisterAmazonProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	return r.registerSinglePlatform(ctx, serviceManager, "amazon")
}

func (r *PlatformProcessorRegistry) registerSinglePlatform(ctx context.Context, serviceManager *ServiceManager, platform string) error {
	r.rabbitmqClient = serviceManager.GetClient()

	if !r.isPlatformEnabled(platform) {
		r.logger.Debugf("skipping %s platform registration", strings.ToUpper(platform))
		return nil
	}

	for _, registration := range r.buildProcessorRegistrations() {
		if registration.name != platform {
			continue
		}
		needsAmazon := registration.needsAmazon || PlatformUsesLocalFetcher(r.config, platform)
		if err := r.initializeSharedResources(needsAmazon); err != nil {
			return err
		}
		return registration.register(ctx, r, serviceManager)
	}

	return fmt.Errorf("unsupported platform: %s", platform)
}

func (r *PlatformProcessorRegistry) anyRegistrationNeedsAmazon(registrations []processorRegistration) bool {
	for _, registration := range registrations {
		if r.isPlatformEnabled(registration.name) && (registration.needsAmazon || PlatformUsesLocalFetcher(r.config, registration.name)) {
			return true
		}
	}
	return false
}

func (r *PlatformProcessorRegistry) isPlatformEnabled(platform string) bool {
	return containsPlatform(r.enabledPlatforms, platform)
}

func (r *PlatformProcessorRegistry) runtimeContext() PlatformRuntimeContext {
	return PlatformRuntimeContext{
		Config:           r.config,
		Logger:           r.logger,
		ManagementClient: r.managementClient,
		CrawlSource:      r.sharedCrawlSource,
		ProductFetcher:   r.sharedProductFetcher,
		RabbitMQClient:   r.rabbitmqClient,
	}
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

func getEnabledPlatformsFromModules(cfg *config.Config, modules []PlatformModule) []string {
	platforms := make([]string, 0)
	for _, module := range modules {
		if module.Enabled(cfg) {
			platforms = append(platforms, module.Name())
		}
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

func (r *PlatformProcessorRegistry) GetSharedAmazonProcessor() *amazon.AmazonProcessor {
	return r.sharedCrawlSource
}

func (r *PlatformProcessorRegistry) GetSharedCrawlSource() *amazon.AmazonProcessor {
	return r.sharedCrawlSource
}

func (r *PlatformProcessorRegistry) GetSharedProductFetcher() appfetcher.ProductFetcher {
	return r.sharedProductFetcher
}

func (r *PlatformProcessorRegistry) GetManagementClient() *management.ClientManager {
	return r.managementClient
}
