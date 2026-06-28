package consumer

import (
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type platformModuleRegistrarFactory func(logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) platformModuleRegistrar

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
		newRegistrar: func(logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) platformModuleRegistrar {
			return newPlatformModuleRegistrar(cfg, logger, serviceManager, resources)
		},
	}
}

func (d PlatformProcessorRegistryDependencies) ResolvePlatformModule(platform string) (PlatformModule, error) {
	return d.catalog.resolve(platform)
}

type CrawlerRegistryDependencies struct {
	amazonCrawlerCreator   amazonCrawlerCreator
	productFetcherProvider productFetcherProvider
}

func NewCrawlerRegistryDependencies(
	amazonCrawlerCreator func(cfg *config.Config, logger *logrus.Logger) runner.CrawlSource,
	productFetcherProvider func(cfg *config.Config, logger *logrus.Logger, crawlSource runner.CrawlSource) (*product.ProductFetcher, error),
) CrawlerRegistryDependencies {
	return CrawlerRegistryDependencies{
		amazonCrawlerCreator:   amazonCrawlerCreator,
		productFetcherProvider: productFetcherProvider,
	}
}
