package consumer

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/ports/managementapi"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type processorRegistration struct {
	module      PlatformModule
	needsAmazon bool
	register    func(context.Context, *PlatformProcessorRegistry, *ServiceManager) error
}

type PlatformProcessorRegistry struct {
	config                  *config.Config
	logger                  *logrus.Logger
	managementClient        *management.ClientManager
	rawJSONDataClient       product.RawJsonDataClient
	storeAPI                managementapi.StoreAPI
	schedulerRuntime        runner.SchedulerRuntimeProvider
	schedulerFactoryRuntime SchedulerFactoryRuntime
	processorRuntime        ProcessorRuntime
	sharedCrawlSource       runner.CrawlSource
	sharedProductFetcher    appfetcher.ProductFetcher
	rabbitmqClient          *rabbitmq.Client
	enabledPlatforms        []string
	sharedResourceProvider  SharedResourceProvider
	platformModules         []PlatformModule
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
	return r.RegisterPlatforms(ctx, serviceManager, r.enabledPlatforms...)
}

func (r *PlatformProcessorRegistry) buildProcessorRegistrations() []processorRegistration {
	registrations := make([]processorRegistration, 0, len(r.platformModules))
	for _, module := range r.platformModules {
		module := module
		registrations = append(registrations, processorRegistration{
			module:      module,
			needsAmazon: module.NeedsAmazon(r.config),
			register: func(ctx context.Context, registry *PlatformProcessorRegistry, serviceManager *ServiceManager) error {
				registry.logger.Infof("registering %s processor", strings.ToUpper(module.Name()))
				if err := module.RegisterConsumer(ctx, registry.runtimeContext(serviceManager, nil), serviceManager); err != nil {
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
	r.rawJSONDataClient = resources.RawJSONDataClient
	r.storeAPI = resources.StoreAPI
	r.schedulerRuntime = resources.SchedulerRuntime
	r.schedulerFactoryRuntime = resources.SchedulerFactoryRuntime
	r.processorRuntime = resources.ProcessorRuntime
	r.sharedCrawlSource = resources.CrawlSource
	r.sharedProductFetcher = resources.ProductFetcher
	r.logger.Info("shared resources initialized")
	return nil
}

func (r *PlatformProcessorRegistry) RegisterPlatforms(ctx context.Context, serviceManager *ServiceManager, platforms ...string) error {
	modules, err := r.resolveModules(platforms)
	if err != nil {
		return err
	}

	r.logger.Infof("registering platform processors: %v", moduleNames(modules))
	r.rabbitmqClient = serviceManager.GetClient()
	if r.rabbitmqClient != nil {
		r.logger.Info("RabbitMQ client available for distributed fetching")
	} else {
		r.logger.Warn("RabbitMQ client unavailable; distributed fetching is unavailable")
	}

	registrations := r.buildProcessorRegistrationsForModules(modules)
	if err := r.initializeSharedResources(r.anyRegistrationNeedsAmazon(registrations)); err != nil {
		return fmt.Errorf("initialize shared resources: %w", err)
	}

	for _, registration := range registrations {
		if err := registration.register(ctx, r, serviceManager); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformProcessorRegistry) anyRegistrationNeedsAmazon(registrations []processorRegistration) bool {
	for _, registration := range registrations {
		name := registration.module.Name()
		if r.isPlatformEnabled(name) && (registration.needsAmazon || PlatformUsesLocalFetcher(r.config, name)) {
			return true
		}
	}
	return false
}

func (r *PlatformProcessorRegistry) isPlatformEnabled(platform string) bool {
	return containsPlatform(r.enabledPlatforms, platform)
}

func (r *PlatformProcessorRegistry) runtimeContext(
	serviceManager *ServiceManager,
	schedulerBuilder SchedulerDependenciesBuilder,
) PlatformRuntimeContext {
	return PlatformRuntimeContext{
		Config:                  r.config,
		Logger:                  r.logger,
		ManagementClient:        r.managementClient,
		RawJSONDataClient:       r.rawJSONDataClient,
		StoreAPI:                r.storeAPI,
		SchedulerRuntime:        r.schedulerRuntime,
		SchedulerFactoryRuntime: r.schedulerFactoryRuntime,
		ProcessorRuntime:        r.processorRuntime,
		CrawlSource:             r.sharedCrawlSource,
		ProductFetcher:          r.sharedProductFetcher,
		RabbitMQClient:          r.rabbitmqClient,
		ServiceManager:          serviceManager,
		SchedulerBuilder:        schedulerBuilder,
	}
}

func (r *PlatformProcessorRegistry) RuntimeContext(
	serviceManager *ServiceManager,
	schedulerBuilder SchedulerDependenciesBuilder,
) PlatformRuntimeContext {
	return r.runtimeContext(serviceManager, schedulerBuilder)
}

func (r *PlatformProcessorRegistry) ResolvePlatformModule(platform string) (PlatformModule, error) {
	module, ok := r.findModule(platform)
	if !ok {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
	if !r.isPlatformEnabled(platform) {
		return nil, fmt.Errorf("%s platform is not enabled", strings.ToUpper(platform))
	}
	return module, nil
}

func (r *PlatformProcessorRegistry) buildProcessorRegistrationsForModules(modules []PlatformModule) []processorRegistration {
	registrations := make([]processorRegistration, 0, len(modules))
	for _, registration := range r.buildProcessorRegistrations() {
		for _, module := range modules {
			if registration.module.Name() == module.Name() {
				registrations = append(registrations, registration)
				break
			}
		}
	}
	return registrations
}

func (r *PlatformProcessorRegistry) resolveModules(platforms []string) ([]PlatformModule, error) {
	if len(platforms) == 0 {
		return r.enabledModules(), nil
	}

	modules := make([]PlatformModule, 0, len(platforms))
	seen := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		normalized := strings.ToLower(strings.TrimSpace(platform))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		module, err := r.ResolvePlatformModule(normalized)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
		seen[normalized] = struct{}{}
	}
	return modules, nil
}

func (r *PlatformProcessorRegistry) enabledModules() []PlatformModule {
	modules := make([]PlatformModule, 0, len(r.platformModules))
	for _, module := range r.platformModules {
		if r.isPlatformEnabled(module.Name()) {
			modules = append(modules, module)
		}
	}
	return modules
}

func (r *PlatformProcessorRegistry) findModule(platform string) (PlatformModule, bool) {
	for _, module := range r.platformModules {
		if strings.EqualFold(module.Name(), platform) {
			return module, true
		}
	}
	return nil, false
}

func moduleNames(modules []PlatformModule) []string {
	names := make([]string, 0, len(modules))
	for _, module := range modules {
		names = append(names, module.Name())
	}
	return names
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

func (r *PlatformProcessorRegistry) GetSharedCrawlSource() runner.CrawlSource {
	return r.sharedCrawlSource
}

func (r *PlatformProcessorRegistry) GetSharedProductFetcher() appfetcher.ProductFetcher {
	return r.sharedProductFetcher
}

func (r *PlatformProcessorRegistry) GetListingRuntimeHealthValidator() ListingRuntimeHealthValidator {
	return r.managementClient
}
