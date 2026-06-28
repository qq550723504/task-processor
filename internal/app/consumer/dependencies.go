package consumer

import "task-processor/internal/core/config"

type PlatformProcessorRegistryDependencies struct {
	PlatformModules []PlatformModule
	Catalog         PlatformModuleCatalog
	ResourceNeeds   PlatformResourceNeedsResolver
}

func NewPlatformProcessorRegistryDependencies(cfg *config.Config, platformsStr string, modules []PlatformModule) PlatformProcessorRegistryDependencies {
	catalog := NewPlatformModuleCatalog(cfg, platformsStr, modules)
	return PlatformProcessorRegistryDependencies{
		PlatformModules: modules,
		Catalog:         catalog,
		ResourceNeeds:   NewPlatformResourceNeedsResolver(cfg, catalog),
	}
}

type CrawlerRegistryDependencies struct {
	AmazonCrawlerCreator   AmazonCrawlerCreator
	ProductFetcherProvider ProductFetcherProvider
}
