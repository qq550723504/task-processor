package consumer

type PlatformProcessorRegistryDependencies struct {
	PlatformModules []PlatformModule
}

type CrawlerRegistryDependencies struct {
	AmazonCrawlerCreator   AmazonCrawlerCreator
	ProductFetcherProvider ProductFetcherProvider
}
