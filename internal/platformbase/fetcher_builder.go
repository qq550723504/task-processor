package platformbase

import (
	"task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	domainProduct "task-processor/internal/product"
)

type ProductFetcherBuilder interface {
	Build(
		rawJsonDataClient domainProduct.RawJsonDataClient,
		amazonConfig *config.AmazonConfig,
		amazonProcessor AmazonCrawler,
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
	amazonProcessor AmazonCrawler,
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
		amazonProcessor,
		rabbitmqClient,
	)
}
