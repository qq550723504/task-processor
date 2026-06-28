package consumer

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type PlatformProcessorRegistry struct {
	config  *config.Config
	logger  *logrus.Logger
	catalog PlatformModuleCatalog
}

func NewPlatformProcessorRegistry(cfg *config.Config, logger *logrus.Logger, platformsStr string, deps PlatformProcessorRegistryDependencies) *PlatformProcessorRegistry {
	catalog := NewPlatformModuleCatalog(cfg, platformsStr, deps.PlatformModules)

	logger.Infof("enabled platforms: %v", catalog.EnabledPlatformNames())

	return &PlatformProcessorRegistry{
		config:  cfg,
		logger:  logger,
		catalog: catalog,
	}
}

func (r *PlatformProcessorRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources) error {
	return r.RegisterPlatforms(ctx, serviceManager, resources, r.catalog.EnabledPlatformNames()...)
}

func (r *PlatformProcessorRegistry) RegisterPlatforms(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources, platforms ...string) error {
	modules, err := r.catalog.ResolveMany(platforms...)
	if err != nil {
		return err
	}

	r.logger.Infof("registering platform processors: %v", platformModuleNames(modules))
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
	modules, err := r.catalog.ResolveMany(platforms...)
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
		if module.NeedsAmazon(r.config) || PlatformUsesLocalFetcher(r.config, name) {
			return true
		}
	}
	return false
}
