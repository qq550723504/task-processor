package consumer

import (
	"context"

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

func (r *PlatformProcessorRegistry) RegisterAllProcessors(ctx context.Context, services PlatformRegistrationServices, resources SharedResources) error {
	return r.RegisterPlatforms(ctx, services, resources, r.catalog.enabledPlatformNames()...)
}

func (r *PlatformProcessorRegistry) RegisterPlatforms(ctx context.Context, services PlatformRegistrationServices, resources SharedResources, platforms ...string) error {
	modules, err := r.catalog.resolveMany(platforms...)
	if err != nil {
		return err
	}

	r.logger.Infof("registering platform processors: %v", platformModuleNames(modules))
	services.LogDistributedFetchingAvailability(r.logger)

	r.logger.Info("shared resources initialized")

	registrar := r.newRegistrar(r.logger, services)
	for _, module := range modules {
		if err := registrar.register(ctx, module, resources); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformProcessorRegistry) SharedResourceNeeds(platforms ...string) (SharedResourceNeeds, error) {
	return r.resourceNeed.resolve(platforms...)
}
