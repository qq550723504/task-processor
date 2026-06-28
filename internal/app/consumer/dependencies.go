package consumer

import (
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type PlatformModuleRegistrarFactory func(logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) PlatformModuleRegistrar

type PlatformProcessorRegistryDependencies struct {
	catalog       PlatformModuleCatalog
	resourceNeeds PlatformResourceNeedsResolver
	newRegistrar  PlatformModuleRegistrarFactory
}

func NewPlatformProcessorRegistryDependencies(cfg *config.Config, platformsStr string, modules []PlatformModule) PlatformProcessorRegistryDependencies {
	catalog := NewPlatformModuleCatalog(cfg, platformsStr, modules)
	return PlatformProcessorRegistryDependencies{
		catalog:       catalog,
		resourceNeeds: NewPlatformResourceNeedsResolver(cfg, catalog),
		newRegistrar: func(logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) PlatformModuleRegistrar {
			return NewPlatformModuleRegistrar(cfg, logger, serviceManager, resources)
		},
	}
}

func (d PlatformProcessorRegistryDependencies) ResolvePlatformModule(platform string) (PlatformModule, error) {
	return d.catalog.Resolve(platform)
}

type CrawlerRegistryDependencies struct {
	AmazonCrawlerCreator   AmazonCrawlerCreator
	ProductFetcherProvider ProductFetcherProvider
}
