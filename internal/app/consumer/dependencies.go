package consumer

import (
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type platformModuleRegistrarFactory func(logger *logrus.Logger, services PlatformRegistrationServices) platformModuleRegistrar

type PlatformProcessorRegistryDependencies struct {
	catalog       platformModuleCatalog
	resourceNeeds platformResourceNeedsResolver
	newRegistrar  platformModuleRegistrarFactory
}

func NewPlatformProcessorRegistryDependencies(cfg *config.Config, platformsStr string, modules []PlatformModule) PlatformProcessorRegistryDependencies {
	catalog := newPlatformModuleCatalog(cfg, platformsStr, modules)
	return PlatformProcessorRegistryDependencies{
		catalog:       catalog,
		resourceNeeds: newPlatformResourceNeedsResolver(cfg, catalog),
		newRegistrar: func(logger *logrus.Logger, services PlatformRegistrationServices) platformModuleRegistrar {
			return newPlatformModuleRegistrar(cfg, logger, services)
		},
	}
}

func (d PlatformProcessorRegistryDependencies) ResolvePlatformModule(platform string) (PlatformModule, error) {
	return d.catalog.resolve(platform)
}
