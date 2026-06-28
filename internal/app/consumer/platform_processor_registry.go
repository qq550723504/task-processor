package consumer

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type PlatformProcessorRegistry struct {
	config          *config.Config
	logger          *logrus.Logger
	selection       platformSelection
	platformModules []PlatformModule
}

func NewPlatformProcessorRegistry(cfg *config.Config, logger *logrus.Logger, platformsStr string, deps PlatformProcessorRegistryDependencies) *PlatformProcessorRegistry {
	selection := newPlatformSelection(cfg, platformsStr, deps.PlatformModules)

	logger.Infof("enabled platforms: %v", selection.names())

	return &PlatformProcessorRegistry{
		config:          cfg,
		logger:          logger,
		selection:       selection,
		platformModules: deps.PlatformModules,
	}
}

func (r *PlatformProcessorRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources) error {
	return r.RegisterPlatforms(ctx, serviceManager, resources, r.selection.names()...)
}

func (r *PlatformProcessorRegistry) RegisterPlatforms(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources, platforms ...string) error {
	modules, err := r.resolveModules(platforms)
	if err != nil {
		return err
	}

	r.logger.Infof("registering platform processors: %v", moduleNames(modules))
	if serviceManager.GetClient() != nil {
		r.logger.Info("RabbitMQ client available for distributed fetching")
	} else {
		r.logger.Warn("RabbitMQ client unavailable; distributed fetching is unavailable")
	}

	if resources == nil {
		return fmt.Errorf("shared resources not configured")
	}
	r.logger.Info("shared resources initialized")

	for _, module := range modules {
		if err := r.registerPlatformModule(ctx, serviceManager, resources, module); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformProcessorRegistry) registerPlatformModule(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources, module PlatformModule) error {
	r.logger.Infof("registering %s processor", strings.ToUpper(module.Name()))
	runtimeContext := BuildPlatformRuntimeContext(PlatformRuntimeContextInput{
		Config:         r.config,
		Logger:         r.logger,
		Resources:      resources,
		ServiceManager: serviceManager,
	})
	if err := module.RegisterConsumer(ctx, runtimeContext, serviceManager); err != nil {
		return err
	}
	r.logger.Infof("%s processor registered", strings.ToUpper(module.Name()))
	return nil
}

func (r *PlatformProcessorRegistry) SharedResourceNeeds(platforms ...string) (SharedResourceNeeds, error) {
	modules, err := r.resolveModules(platforms)
	if err != nil {
		return SharedResourceNeeds{}, err
	}
	return SharedResourceNeeds{
		NeedAmazonCrawler: r.anyModuleNeedsAmazon(modules),
	}, nil
}

func (r *PlatformProcessorRegistry) anyModuleNeedsAmazon(modules []PlatformModule) bool {
	for _, module := range modules {
		name := module.Name()
		if r.selection.isEnabled(name) && (module.NeedsAmazon(r.config) || PlatformUsesLocalFetcher(r.config, name)) {
			return true
		}
	}
	return false
}

func (r *PlatformProcessorRegistry) ResolvePlatformModule(platform string) (PlatformModule, error) {
	module, ok := r.findModule(platform)
	if !ok {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
	if !r.selection.isEnabled(platform) {
		return nil, fmt.Errorf("%s platform is not enabled", strings.ToUpper(platform))
	}
	return module, nil
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
	return r.selection.enabledModules(r.platformModules)
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
