package consumer

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type PlatformProcessorRegistry struct {
	config        *config.Config
	logger        *logrus.Logger
	catalog       PlatformModuleCatalog
	resourceNeeds PlatformResourceNeedsResolver
}

func NewPlatformProcessorRegistry(cfg *config.Config, logger *logrus.Logger, platformsStr string, deps PlatformProcessorRegistryDependencies) *PlatformProcessorRegistry {
	catalog := NewPlatformModuleCatalog(cfg, platformsStr, deps.PlatformModules)
	resourceNeeds := NewPlatformResourceNeedsResolver(cfg, catalog)

	logger.Infof("enabled platforms: %v", catalog.EnabledPlatformNames())

	return &PlatformProcessorRegistry{
		config:        cfg,
		logger:        logger,
		catalog:       catalog,
		resourceNeeds: resourceNeeds,
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

	registrar := NewPlatformModuleRegistrar(r.config, r.logger, serviceManager, resources)
	for _, module := range modules {
		if err := registrar.Register(ctx, module); err != nil {
			return err
		}
	}

	r.logger.Info("platform processors registered")
	return nil
}

func (r *PlatformProcessorRegistry) SharedResourceNeeds(platforms ...string) (SharedResourceNeeds, error) {
	return r.resourceNeeds.Resolve(platforms...)
}
