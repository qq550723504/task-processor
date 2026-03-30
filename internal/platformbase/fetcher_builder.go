package platformbase

import (
	"task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	domainProduct "task-processor/internal/product"
)

type ProductFetcherBuilder interface {
	Build(
		rawJsonDataClient domainProduct.RawJsonDataClient,
		amazonConfig *config.AmazonConfig,
		crawlSource ports.CrawlSource,
		rabbitmqClient *rabbitmq.Client,
	) (fetcher.ProductFetcher, error)
}

type DefaultProductFetcherBuilder struct {
	factory *fetcher.FetcherFactory
}

func NewDefaultProductFetcherBuilder() *DefaultProductFetcherBuilder {
	return &DefaultProductFetcherBuilder{factory: fetcher.NewFetcherFactory()}
}

func (b *DefaultProductFetcherBuilder) Build(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (fetcher.ProductFetcher, error) {
	fetcherType := fetcher.LocalFetcher
	if rabbitmqClient != nil {
		fetcherType = fetcher.DistributedFetcher
	}

	return b.factory.CreateFetcher(
		fetcherType,
		rawJsonDataClient,
		amazonConfig,
		crawlSource,
		rabbitmqClient,
	)
}

type boundProductFetcherBuilder struct {
	base        ProductFetcherBuilder
	crawlSource ports.CrawlSource
}

// BindProductFetcherBuilder returns a builder that reuses a pre-bound product source.
func BindProductFetcherBuilder(base ProductFetcherBuilder, crawlSource ports.CrawlSource) ProductFetcherBuilder {
	if base == nil {
		base = NewDefaultProductFetcherBuilder()
	}
	return &boundProductFetcherBuilder{
		base:        base,
		crawlSource: crawlSource,
	}
}

func (b *boundProductFetcherBuilder) Build(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (fetcher.ProductFetcher, error) {
	if crawlSource == nil {
		crawlSource = b.crawlSource
	}
	return b.base.Build(rawJsonDataClient, amazonConfig, crawlSource, rabbitmqClient)
}
