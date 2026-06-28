package consumer

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

type processorRegistration struct {
	module      PlatformModule
	needsAmazon bool
	register    func(context.Context, *PlatformProcessorRegistry, *ServiceManager) error
}

type PlatformProcessorRegistry struct {
	config           *config.Config
	logger           *logrus.Logger
	sharedResources  *SharedResources
	rabbitmqClient   *rabbitmq.Client
	enabledPlatforms []string
	platformModules  []PlatformModule
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
		config:           cfg,
		logger:           logger,
		enabledPlatforms: enabledPlatforms,
		platformModules:  deps.PlatformModules,
	}
}

func (r *PlatformProcessorRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources) error {
	return r.RegisterPlatforms(ctx, serviceManager, resources, r.enabledPlatforms...)
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

func (r *PlatformProcessorRegistry) useSharedResources(resources *SharedResources) error {
	if resources == nil {
		return fmt.Errorf("shared resources not configured")
	}

	r.sharedResources = resources
	r.logger.Info("shared resources initialized")
	return nil
}

func (r *PlatformProcessorRegistry) RegisterPlatforms(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources, platforms ...string) error {
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
	if err := r.useSharedResources(resources); err != nil {
		return err
	}

	for _, registration := range registrations {
		if err := registration.register(ctx, r, serviceManager); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformProcessorRegistry) SharedResourceNeeds(platforms ...string) (SharedResourceNeeds, error) {
	modules, err := r.resolveModules(platforms)
	if err != nil {
		return SharedResourceNeeds{}, err
	}
	registrations := r.buildProcessorRegistrationsForModules(modules)
	return SharedResourceNeeds{
		NeedAmazonCrawler: r.anyRegistrationNeedsAmazon(registrations),
	}, nil
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
	resources := r.runtimeResources()
	return PlatformRuntimeContext{
		Config:                             r.config,
		Logger:                             r.logger,
		ListingRuntimeImportTaskRepository: resources.ListingRuntimeImportTaskRepository,
		RawJSONDataClient:                  resources.RawJSONDataClient,
		StoreAPI:                           resources.StoreAPI,
		SchedulerRuntime:                   resources.SchedulerRuntime,
		SchedulerFactoryRuntime:            resources.SchedulerFactoryRuntime,
		ProcessorRuntime:                   resources.ProcessorRuntime,
		CrawlSource:                        resources.CrawlSource,
		ProductFetcher:                     resources.ProductFetcher,
		RabbitMQClient:                     r.rabbitmqClient,
		ServiceManager:                     serviceManager,
		SchedulerBuilder:                   schedulerBuilder,
	}
}

func (r *PlatformProcessorRegistry) runtimeResources() SharedResources {
	if r == nil || r.sharedResources == nil {
		return SharedResources{}
	}
	return *r.sharedResources
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
	return r.runtimeResources().CrawlSource
}

func (r *PlatformProcessorRegistry) GetSharedProductFetcher() appfetcher.ProductFetcher {
	return r.runtimeResources().ProductFetcher
}
