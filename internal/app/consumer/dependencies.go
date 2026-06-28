package consumer

import (
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type PlatformModuleRegistrarFactory func(logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) PlatformModuleRegistrar

type PlatformProcessorRegistryDependencies struct {
	Catalog       PlatformModuleCatalog
	ResourceNeeds PlatformResourceNeedsResolver
	NewRegistrar  PlatformModuleRegistrarFactory
}

func NewPlatformProcessorRegistryDependencies(cfg *config.Config, platformsStr string, modules []PlatformModule) PlatformProcessorRegistryDependencies {
	catalog := NewPlatformModuleCatalog(cfg, platformsStr, modules)
	return PlatformProcessorRegistryDependencies{
		Catalog:       catalog,
		ResourceNeeds: NewPlatformResourceNeedsResolver(cfg, catalog),
		NewRegistrar: func(logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) PlatformModuleRegistrar {
			return NewPlatformModuleRegistrar(cfg, logger, serviceManager, resources)
		},
	}
}

type CrawlerRegistryDependencies struct {
	AmazonCrawlerCreator   AmazonCrawlerCreator
	ProductFetcherProvider ProductFetcherProvider
}
