package consumer

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

type PlatformProcessorRegistry struct {
	logger       *logrus.Logger
	catalog      platformModuleCatalog
	resourceNeed platformResourceNeedsResolver
	newRegistrar platformModuleRegistrarFactory
}

func NewPlatformProcessorRegistry(logger *logrus.Logger, deps PlatformProcessorRegistryDependencies) *PlatformProcessorRegistry {
	logger.Infof("enabled platforms: %v", deps.catalog.enabledPlatformNames())

	return &PlatformProcessorRegistry{
		logger:       logger,
		catalog:      deps.catalog,
		resourceNeed: deps.resourceNeeds,
		newRegistrar: deps.newRegistrar,
	}
}

func (r *PlatformProcessorRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources) error {
	return r.RegisterPlatforms(ctx, serviceManager, resources, r.catalog.enabledPlatformNames()...)
}

func (r *PlatformProcessorRegistry) RegisterPlatforms(ctx context.Context, serviceManager *ServiceManager, resources *SharedResources, platforms ...string) error {
	modules, err := r.catalog.resolveMany(platforms...)
	if err != nil {
		return err
	}

	r.logger.Infof("registering platform processors: %v", platformModuleNames(modules))
	serviceManager.LogDistributedFetchingAvailability(r.logger)

	if resources == nil {
		return fmt.Errorf("shared resources not configured")
	}
	r.logger.Info("shared resources initialized")

	resourcesValue := *resources
	registrar := r.newRegistrar(r.logger, serviceManager)
	for _, module := range modules {
		if err := registrar.register(ctx, module, resourcesValue); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformProcessorRegistry) SharedResourceNeeds(platforms ...string) (SharedResourceNeeds, error) {
	return r.resourceNeed.resolve(platforms...)
}
