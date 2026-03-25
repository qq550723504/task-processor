package consumer

type PlatformRegistryDependencies struct {
	ProcessorCreators      ProcessorCreators
	SharedResourceProvider SharedResourceProvider
}

type CrawlerRegistryDependencies struct {
	AmazonCrawlerCreator   AmazonCrawlerCreator
	ProductFetcherProvider ProductFetcherProvider
}
