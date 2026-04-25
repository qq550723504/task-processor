package consumer

type PlatformProcessorRegistryDependencies struct {
	SharedResourceProvider SharedResourceProvider
	PlatformModules        []PlatformModule
}

type CrawlerRegistryDependencies struct {
	AmazonCrawlerCreator   AmazonCrawlerCreator
	ProductFetcherProvider ProductFetcherProvider
}
